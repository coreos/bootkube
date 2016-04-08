package bootkube

import (
	"fmt"
	"io/ioutil"
	"net"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"golang.org/x/crypto/ssh"
	apiapp "k8s.io/kubernetes/cmd/kube-apiserver/app"
	apiserver "k8s.io/kubernetes/cmd/kube-apiserver/app/options"
	cmapp "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	controller "k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	clientcmdapi "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/wait"
	schedapp "k8s.io/kubernetes/plugin/cmd/kube-scheduler/app"
	scheduler "k8s.io/kubernetes/plugin/cmd/kube-scheduler/app/options"
)

const (
	localAPIServerAddr         = "127.0.0.1:6443"
	localAPIServerInsecureAddr = "127.0.0.1:8080"
	remoteAPIServerAddr        = "0.0.0.0:6443"
	localEtcdServerAddr        = "127.0.0.1:2379"
)

type Config struct {
	SSHUser           string
	SSHKeyFile        string
	APIServerKey      string
	APIServerCert     string
	CACert            string
	ServiceAccountKey string
	TokenAuth         string
	RemoteAddr        string
	RemoteEtcdAddr    string
	ManifestDir       string
}

type bootkube struct {
	apiServer      *apiserver.APIServer
	controller     *controller.CMServer
	scheduler      *scheduler.SchedulerServer
	clientConfig   clientcmd.ClientConfig
	sshConfig      *ssh.ClientConfig
	remoteAddr     string
	remoteEtcdAddr string
	manifestDir    string
}

func NewBootkube(config Config) (*bootkube, error) {
	apiServer := apiserver.NewAPIServer()
	fs := pflag.NewFlagSet("apiserver", pflag.ExitOnError)
	apiServer.AddFlags(fs)
	fs.Parse([]string{
		"--allow-privileged=true",
		"--secure-port=6443",
		"--tls-private-key-file=" + config.APIServerKey,
		"--tls-cert-file=" + config.APIServerCert,
		"--etcd-servers=http://" + localEtcdServerAddr,
		"--service-cluster-ip-range=10.3.0.0/24",
		"--service-account-key-file=" + config.ServiceAccountKey,
		"--token-auth-file=" + config.TokenAuth,
		"--admission-control=ServiceAccount",
		"--runtime-config=extensions/v1beta1/deployments=true,extensions/v1beta1/daemonsets=true",
	})

	cmServer := controller.NewCMServer()
	fs = pflag.NewFlagSet("controllermanager", pflag.ExitOnError)
	cmServer.AddFlags(fs)
	fs.Parse([]string{
		"--master=" + localAPIServerInsecureAddr,
		"--service-account-private-key-file=" + config.ServiceAccountKey,
		"--root-ca-file=" + config.CACert,
		"--leader-elect=true",
	})

	schedServer := scheduler.NewSchedulerServer()
	fs = pflag.NewFlagSet("scheduler", pflag.ExitOnError)
	schedServer.AddFlags(fs)
	fs.Parse([]string{
		"--master=" + localAPIServerInsecureAddr,
		"--leader-elect=true",
	})

	sshConfig, err := newSSHConfig(config.SSHUser, config.SSHKeyFile)
	if err != nil {
		return nil, err
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{},
		&clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: localAPIServerInsecureAddr}},
	)

	return &bootkube{
		apiServer:      apiServer,
		controller:     cmServer,
		scheduler:      schedServer,
		clientConfig:   clientConfig,
		sshConfig:      sshConfig,
		remoteAddr:     config.RemoteAddr,
		remoteEtcdAddr: config.RemoteEtcdAddr,
		manifestDir:    config.ManifestDir,
	}, nil
}

func (b *bootkube) Run() error {
	tunnel, err := ssh.Dial("tcp", b.remoteAddr, b.sshConfig)
	if err != nil {
		return err
	}
	defer tunnel.Close()

	// Proxy connections from remote end of ssh tunnel to local apiserver
	apiProxy := &Proxy{
		listenAddr: remoteAPIServerAddr,
		listenFunc: tunnel.Listen,
		dialAddr:   localAPIServerAddr,
		dialFunc:   net.Dial,
	}

	// Proxy local connections to remote etcd via ssh tunnel
	etcdProxy := &Proxy{
		listenAddr: localEtcdServerAddr,
		listenFunc: net.Listen,
		dialAddr:   b.remoteEtcdAddr,
		dialFunc:   tunnel.Dial,
	}

	errch := make(chan error)
	go func() { errch <- apiProxy.Run() }()
	go func() { errch <- etcdProxy.Run() }()
	go func() { errch <- apiapp.Run(b.apiServer) }()
	go func() { errch <- cmapp.Run(b.controller) }()
	go func() { errch <- schedapp.Run(b.scheduler) }()
	go func() {
		// Ignore "already exists" object errors
		// TODO(aaron): we should keep retrying on 'already exists' errors
		if err := b.createAssets(); err != nil && !strings.Contains(err.Error(), "already exists") {
			errch <- err
		}
	}()

	// If any of the bootkube services exit, it means they can't initialize and we should exit.
	return <-errch
}

func (b *bootkube) createAssets() error {
	if err := wait.Poll(5*time.Second, 60*time.Second, b.isUp); err != nil {
		return fmt.Errorf("API Server unavailable: %v", err)
	}

	f := cmdutil.NewFactory(b.clientConfig)
	schema, err := f.Validator(true, fmt.Sprintf("~/%s/%s", clientcmd.RecommendedHomeDir, clientcmd.RecommendedSchemaName))
	if err != nil {
		return err
	}
	cmdNamespace, enforceNamespace, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	mapper, typer := f.Object()
	r := resource.NewBuilder(mapper, typer, resource.ClientMapperFunc(f.ClientForMapping), f.Decoder(true)).
		Schema(schema).
		ContinueOnError().
		NamespaceParam(cmdNamespace).DefaultNamespace().
		FilenameParam(enforceNamespace, b.manifestDir).
		Flatten().
		Do()
	err = r.Err()
	if err != nil {
		return err
	}

	count := 0
	err = r.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}

		obj, err := resource.NewHelper(info.Client, info.Mapping).Create(info.Namespace, true, info.Object)
		if err != nil {
			return cmdutil.AddSourceToErr("creating", info.Source, err)
		}
		info.Refresh(obj, true)

		count++
		cmdutil.PrintSuccess(mapper, false, util.GlogWriter{}, info.Mapping.Resource, info.Name, "created")
		return nil
	})
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("no objects passed to create")
	}
	return nil

}

func (b *bootkube) isUp() (bool, error) {
	f := cmdutil.NewFactory(b.clientConfig)
	client, err := f.Client()
	if err != nil {
		return false, fmt.Errorf("failed to create kube client: %v", err)
	}

	// TODO(aaron): we want wait.Poll to retry on failures here (maybe just log err and return false)
	_, err = client.Discovery().ServerVersion()
	return err == nil, err
}

func newSSHConfig(user, keyfile string) (*ssh.ClientConfig, error) {
	buffer, err := ioutil.ReadFile(keyfile)
	if err != nil {
		return nil, err
	}

	signer, err := ssh.ParsePrivateKey(buffer)
	if err != nil {
		return nil, err
	}

	return &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
	}, nil
}
