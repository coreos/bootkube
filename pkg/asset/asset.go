package asset

import (
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"

	"github.com/kubernetes-incubator/bootkube/pkg/tlsutil"
)

const (
	AssetPathSecrets                        = "tls"
	AssetPathCAKey                          = "tls/ca.key"
	AssetPathCACert                         = "tls/ca.crt"
	AssetPathAPIServerKey                   = "tls/apiserver.key"
	AssetPathAPIServerCert                  = "tls/apiserver.crt"
	AssetPathEtcdCA                         = "tls/etcd-ca.crt"
	AssetPathEtcdClientCert                 = "tls/etcd-client.crt"
	AssetPathEtcdClientKey                  = "tls/etcd-client.key"
	AssetPathEtcdPeerCert                   = "tls/etcd-peer.crt"
	AssetPathEtcdPeerKey                    = "tls/etcd-peer.key"
	AssetPathEtcdServerCert                 = "tls/etcd-server.crt"
	AssetPathEtcdServerKey                  = "tls/etcd-server.key"
	AssetPathServiceAccountPrivKey          = "tls/service-account.key"
	AssetPathServiceAccountPubKey           = "tls/service-account.pub"
	AssetPathKubeletKey                     = "tls/kubelet.key"
	AssetPathKubeletCert                    = "tls/kubelet.crt"
	AssetPathKubeConfig                     = "auth/kubeconfig"
	AssetPathManifests                      = "manifests"
	AssetPathKubelet                        = "manifests/kubelet.yaml"
	AssetPathProxy                          = "manifests/kube-proxy.yaml"
	AssetPathKubeFlannel                    = "manifests/kube-flannel.yaml"
	AssetPathKubeFlannelCfg                 = "manifests/kube-flannel-cfg.yaml"
	AssetPathKubeCalico                     = "manifests/kube-calico.yaml"
	AssetPathKubeCalicoCfg                  = "manifests/kube-calico-cfg.yaml"
	AssetPathKubeCalcioSA                   = "manifests/kube-calico-sa.yaml"
	AssetPathKubeCalcioRole                 = "manifests/kube-calico-role.yaml"
	AssetPathKubeCalcioRoleBinding          = "manifests/kube-calico-role-binding.yaml"
	AssetPathAPIServerSecret                = "manifests/kube-apiserver-secret.yaml"
	AssetPathAPIServer                      = "manifests/kube-apiserver.yaml"
	AssetPathControllerManager              = "manifests/kube-controller-manager.yaml"
	AssetPathControllerManagerSecret        = "manifests/kube-controller-manager-secret.yaml"
	AssetPathControllerManagerDisruption    = "manifests/kube-controller-manager-disruption.yaml"
	AssetPathScheduler                      = "manifests/kube-scheduler.yaml"
	AssetPathSchedulerDisruption            = "manifests/kube-scheduler-disruption.yaml"
	AssetPathKubeDNSDeployment              = "manifests/kube-dns-deployment.yaml"
	AssetPathKubeDNSSvc                     = "manifests/kube-dns-svc.yaml"
	AssetPathSystemNamespace                = "manifests/kube-system-ns.yaml"
	AssetPathCheckpointer                   = "manifests/pod-checkpointer.yaml"
	AssetPathEtcdOperator                   = "manifests/etcd-operator.yaml"
	AssetPathSelfHostedEtcdOperatorSecret   = "manifests/etcd-operator-client-tls.yaml"
	AssetPathSelfHostedEtcdMemberPeerSecret = "manifests/etcd-member-peer-tls.yaml"
	AssetPathSelfHostedEtcdMemberCliSecret  = "manifests/etcd-member-client-tls.yaml"
	AssetPathEtcdSvc                        = "manifests/etcd-service.yaml"
	AssetPathKenc                           = "manifests/kube-etcd-network-checkpointer.yaml"
	AssetPathKubeSystemSARoleBinding        = "manifests/kube-system-rbac-role-binding.yaml"
	AssetPathBootstrapManifests             = "bootstrap-manifests"
	AssetPathBootstrapAPIServer             = "bootstrap-manifests/bootstrap-apiserver.yaml"
	AssetPathBootstrapControllerManager     = "bootstrap-manifests/bootstrap-controller-manager.yaml"
	AssetPathBootstrapScheduler             = "bootstrap-manifests/bootstrap-scheduler.yaml"
	AssetPathBootstrapEtcd                  = "bootstrap-manifests/bootstrap-etcd.yaml"
	AssetPathBootstrapEtcdService           = "etcd/bootstrap-etcd-service.json"
	AssetPathMigrateEtcdCluster             = "etcd/migrate-etcd-cluster.json"
)

var (
	BootstrapSecretsDir = "/etc/kubernetes/bootstrap-secrets" // Overridden for testing.
)

// AssetConfig holds all configuration needed when generating
// the default set of assets.
type Config struct {
	EtcdCACert             *x509.Certificate
	EtcdClientCert         *x509.Certificate
	EtcdClientKey          *rsa.PrivateKey
	EtcdServers            []*url.URL
	EtcdUseTLS             bool
	APIServers             []*url.URL
	CACert                 *x509.Certificate
	CAPrivKey              *rsa.PrivateKey
	AltNames               *tlsutil.AltNames
	PodCIDR                *net.IPNet
	ServiceCIDR            *net.IPNet
	APIServiceIP           net.IP
	BootEtcdServiceIP      net.IP
	DNSServiceIP           net.IP
	EtcdServiceIP          net.IP
	EtcdServiceName        string
	SelfHostKubelet        bool
	SelfHostedEtcd         bool
	CalicoNetworkPolicy    bool
	CloudProvider          string
	BootstrapSecretsSubdir string
	Images                 ImageVersions
}

// ImageVersions holds all the images (and their versions) that are rendered into the templates.
type ImageVersions struct {
	Etcd            string
	EtcdOperator    string
	Flannel         string
	FlannelCNI      string
	Calico          string
	CalicoCNI       string
	Hyperkube       string
	Kenc            string
	KubeDNS         string
	KubeDNSMasq     string
	KubeDNSSidecar  string
	PodCheckpointer string
}

// NewDefaultAssets returns a list of default assets, optionally
// configured via a user provided AssetConfig. Default assets include
// TLS assets (certs, keys and secrets), and k8s component manifests.
func NewDefaultAssets(conf Config) (Assets, error) {
	conf.BootstrapSecretsSubdir = path.Base(BootstrapSecretsDir)

	as := newStaticAssets(conf.Images)
	as = append(as, newDynamicAssets(conf)...)

	// Add kube-apiserver service IP
	conf.AltNames.IPs = append(conf.AltNames.IPs, conf.APIServiceIP)

	// Create a CA if none was provided.
	if conf.CACert == nil {
		var err error
		conf.CAPrivKey, conf.CACert, err = newCACert()
		if err != nil {
			return Assets{}, err
		}
	}

	// TLS assets
	tlsAssets, err := newTLSAssets(conf.CACert, conf.CAPrivKey, *conf.AltNames)
	if err != nil {
		return Assets{}, err
	}
	as = append(as, tlsAssets...)

	// etcd TLS assets.
	if conf.EtcdUseTLS {
		if conf.SelfHostedEtcd {
			tlsAssets, err := newSelfHostedEtcdTLSAssets(conf.EtcdServiceIP.String(), conf.BootEtcdServiceIP.String(), conf.CACert, conf.CAPrivKey)
			if err != nil {
				return nil, err
			}
			as = append(as, tlsAssets...)

			secretAssets, err := newSelfHostedEtcdSecretAssets(as)
			if err != nil {
				return nil, err
			}
			as = append(as, secretAssets...)
		} else {
			etcdTLSAssets, err := newEtcdTLSAssets(conf.EtcdCACert, conf.EtcdClientCert, conf.EtcdClientKey, conf.CACert, conf.CAPrivKey, conf.EtcdServers)
			if err != nil {
				return Assets{}, err
			}
			as = append(as, etcdTLSAssets...)
		}
	}

	// K8S kubeconfig
	kubeConfig, err := newKubeConfigAsset(as, conf)
	if err != nil {
		return Assets{}, err
	}
	as = append(as, kubeConfig)

	// K8S APIServer secret
	apiSecret, err := newAPIServerSecretAsset(as, conf.EtcdUseTLS)
	if err != nil {
		return Assets{}, err
	}
	as = append(as, apiSecret)

	// K8S ControllerManager secret
	cmSecret, err := newControllerManagerSecretAsset(as)
	if err != nil {
		return Assets{}, err
	}
	as = append(as, cmSecret)

	return as, nil
}

type Asset struct {
	Name string
	Data []byte
}

type Assets []Asset

func (as Assets) Get(name string) (Asset, error) {
	for _, asset := range as {
		if asset.Name == name {
			return asset, nil
		}
	}
	return Asset{}, fmt.Errorf("asset %q does not exist", name)
}

func (as Assets) MustGet(name string) Asset {
	asset, err := as.Get(name)
	if err != nil {
		panic(err)
	}
	return asset
}

func (as Assets) WriteFiles(path string) error {
	if err := os.Mkdir(path, 0755); err != nil {
		return err
	}
	for _, asset := range as {
		if err := asset.WriteFile(path); err != nil {
			return err
		}
	}
	return nil
}

func (a Asset) WriteFile(path string) error {
	f := filepath.Join(path, a.Name)
	if err := os.MkdirAll(filepath.Dir(f), 0755); err != nil {
		return err
	}
	fmt.Printf("Writing asset: %s\n", f)
	return ioutil.WriteFile(f, a.Data, 0600)
}
