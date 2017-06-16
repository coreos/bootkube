package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"github.com/kubernetes-incubator/bootkube/pkg/bootkube"
	"github.com/kubernetes-incubator/bootkube/pkg/recovery"
	"github.com/kubernetes-incubator/bootkube/pkg/util/etcdutil"

	"github.com/coreos/etcd/clientv3"
	"github.com/spf13/cobra"
)

var (
	cmdRecover = &cobra.Command{
		Use:          "recover",
		Short:        "Recover a self-hosted control plane",
		Long:         "This command reads control plane manifests from a running apiserver or etcd and writes them to recovery-dir. Users can then use `bootkube start` pointed at this recovery-dir to re-the a self-hosted cluster. Please see the project README for more details and examples.",
		PreRunE:      validateRecoverOpts,
		RunE:         runCmdRecover,
		SilenceUsage: true,
	}

	recoverOpts struct {
		recoveryDir         string
		etcdCAPath          string
		etcdCertificatePath string
		etcdPrivateKeyPath  string
		etcdServers         string
		etcdPrefix          string
		kubeConfigPath      string
		podManifestPath     string
		etcdBackupFile      string
	}
)

func init() {
	cmdRoot.AddCommand(cmdRecover)
	cmdRecover.Flags().StringVar(&recoverOpts.recoveryDir, "recovery-dir", "", "Output path for writing recovered cluster assets.")
	cmdRecover.Flags().StringVar(&recoverOpts.etcdCAPath, "etcd-ca-path", "", "Path to an existing PEM encoded CA that will be used for TLS-enabled communication between the apiserver and etcd. Must be used in conjunction with --etcd-certificate-path and --etcd-private-key-path, and must have etcd configured to use TLS with matching secrets.")
	cmdRecover.Flags().StringVar(&recoverOpts.etcdCertificatePath, "etcd-certificate-path", "", "Path to an existing certificate that will be used for TLS-enabled communication between the apiserver and etcd. Must be used in conjunction with --etcd-ca-path and --etcd-private-key-path, and must have etcd configured to use TLS with matching secrets.")
	cmdRecover.Flags().StringVar(&recoverOpts.etcdPrivateKeyPath, "etcd-private-key-path", "", "Path to an existing private key that will be used for TLS-enabled communication between the apiserver and etcd. Must be used in conjunction with --etcd-ca-path and --etcd-certificate-path, and must have etcd configured to use TLS with matching secrets.")
	cmdRecover.Flags().StringVar(&recoverOpts.etcdServers, "etcd-servers", "", "List of etcd server URLs including host:port, comma separated.")
	cmdRecover.Flags().StringVar(&recoverOpts.etcdBackupFile, "etcd-backup-file", "", "Path to the etcd backup file.")
	cmdRecover.Flags().StringVar(&recoverOpts.etcdPrefix, "etcd-prefix", "/registry", "Path prefix to Kubernetes cluster data in etcd.")
	cmdRecover.Flags().StringVar(&recoverOpts.kubeConfigPath, "kubeconfig", "", "Path to kubeconfig for communicating with the cluster.")
	cmdRecover.Flags().StringVar(&recoverOpts.podManifestPath, "pod-manifest-path", "/etc/kubernetes/manifests", "The location where the kubelet is configured to look for static pod manifests. (Only need to be set when recovering from a etcd backup file)")
}

func runCmdRecover(cmd *cobra.Command, args []string) error {
	var err error
	recoverOpts.kubeConfigPath, err = filepath.Abs(recoverOpts.kubeConfigPath)
	if err != nil {
		return err
	}

	var backend recovery.Backend
	switch {
	case recoverOpts.etcdServers != "":
		bootkube.UserOutput("Attempting recovery using etcd cluster at %q...\n", recoverOpts.etcdServers)
		etcdClient, err := createEtcdClient()
		if err != nil {
			return err
		}
		backend = recovery.NewEtcdBackend(etcdClient, recoverOpts.etcdPrefix)
	case recoverOpts.etcdBackupFile != "":
		bootkube.UserOutput("Attempting recovery using etcd backup file at %q...\n", recoverOpts.etcdBackupFile)

		err = recovery.StartRecoveryEtcdForBackup(recoverOpts.podManifestPath, recoverOpts.etcdBackupFile)
		if err != nil {
			return err
		}
		defer func() {
			err = recovery.CleanRecoveryEtcd(recoverOpts.podManifestPath)
			if err != nil {
				bootkube.UserOutput("Failed to cleanup recovery etcd from the podManifestPath %v\n", recoverOpts.podManifestPath)
			}
		}()

		bootkube.UserOutput("Waiting for etcd server to start...\n")

		err = etcdutil.WaitClusterReady(recovery.RecoveryEtcdClientAddr)
		if err != nil {
			return err
		}

		recoverOpts.etcdServers = recovery.RecoveryEtcdClientAddr
		etcdClient, err := createEtcdClient()
		if err != nil {
			return err
		}
		backend = recovery.NewSelfHostedEtcdBackend(etcdClient, recoverOpts.etcdPrefix, recoverOpts.etcdBackupFile)

	default:
		bootkube.UserOutput("Attempting recovery using apiserver at %q...\n", recoverOpts.kubeConfigPath)
		backend, err = recovery.NewAPIServerBackend(recoverOpts.kubeConfigPath)
		if err != nil {
			return err
		}
	}

	as, err := recovery.Recover(context.Background(), backend, recoverOpts.kubeConfigPath)
	if err != nil {
		return err
	}
	return as.WriteFiles(recoverOpts.recoveryDir)
}

func validateRecoverOpts(cmd *cobra.Command, args []string) error {
	if recoverOpts.recoveryDir == "" {
		return errors.New("missing required flag: --recovery-dir")
	}
	if (recoverOpts.etcdCAPath != "" || recoverOpts.etcdCertificatePath != "" || recoverOpts.etcdPrivateKeyPath != "") && (recoverOpts.etcdCAPath == "" || recoverOpts.etcdCertificatePath == "" || recoverOpts.etcdPrivateKeyPath == "") {
		return errors.New("you must specify either all or none of --etcd-ca-path, --etcd-certificate-path, and --etcd-private-key-path")
	}
	if recoverOpts.etcdPrefix == "" {
		return errors.New("missing required flag: --etcd-prefix")
	}
	if recoverOpts.kubeConfigPath == "" {
		return errors.New("missing required flag: --kubeconfig")
	}
	if recoverOpts.etcdBackupFile != "" && recoverOpts.podManifestPath == "" {
		return errors.New("missing required flag: --pod-manifest-path (--etcd-backup-file flag is specified)")
	}
	return nil
}

func createEtcdClient() (*clientv3.Client, error) {
	cfg := clientv3.Config{
		Endpoints:   strings.Split(recoverOpts.etcdServers, ","),
		DialTimeout: 5 * time.Second,
	}
	if recoverOpts.etcdCAPath != "" {
		clientCert, err := tls.LoadX509KeyPair(recoverOpts.etcdCertificatePath, recoverOpts.etcdPrivateKeyPath)
		if err != nil {
			return nil, err
		}
		roots := x509.NewCertPool()
		etcdCA, err := ioutil.ReadFile(recoverOpts.etcdCAPath)
		if err != nil {
			return nil, err
		}
		if ok := roots.AppendCertsFromPEM(etcdCA); !ok {
			return nil, fmt.Errorf("error processing --etcd-ca-file %s", recoverOpts.etcdCAPath)
		}
		cfg.TLS = &tls.Config{
			Certificates: []tls.Certificate{clientCert},
			RootCAs:      roots,
		}
	}
	return clientv3.New(cfg)
}
