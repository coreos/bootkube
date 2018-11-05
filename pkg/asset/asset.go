package asset

import (
	"crypto/rsa"
	"crypto/x509"
	"errors"
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
	AssetPathEtcdClientCA                   = "tls/etcd-client-ca.crt"
	AssetPathEtcdClientCert                 = "tls/etcd-client.crt"
	AssetPathEtcdClientKey                  = "tls/etcd-client.key"
	AssetPathEtcdServerCA                   = "tls/etcd/server-ca.crt"
	AssetPathEtcdServerCert                 = "tls/etcd/server.crt"
	AssetPathEtcdServerKey                  = "tls/etcd/server.key"
	AssetPathEtcdPeerCA                     = "tls/etcd/peer-ca.crt"
	AssetPathEtcdPeerCert                   = "tls/etcd/peer.crt"
	AssetPathEtcdPeerKey                    = "tls/etcd/peer.key"
	AssetPathServiceAccountPrivKey          = "tls/service-account.key"
	AssetPathServiceAccountPubKey           = "tls/service-account.pub"
	AssetPathAdminKey                       = "tls/admin.key"
	AssetPathAdminCert                      = "tls/admin.crt"
	AssetPathAdminKubeConfig                = "auth/kubeconfig"
	AssetPathKubeletKubeConfig              = "auth/kubeconfig-kubelet"
	AssetPathManifests                      = "manifests"
	AssetPathKubeConfigInCluster            = "manifests/kubeconfig-in-cluster.yaml"
	AssetPathKubeletBootstrapToken          = "manifests/kubelet-bootstrap-token.yaml"
	AssetPathProxy                          = "manifests/kube-proxy.yaml"
	AssetPathProxySA                        = "manifests/kube-proxy-sa.yaml"
	AssetPathProxyRoleBinding               = "manifests/kube-proxy-role-binding.yaml"
	AssetPathFlannel                        = "manifests/flannel.yaml"
	AssetPathFlannelCfg                     = "manifests/flannel-cfg.yaml"
	AssetPathFlannelClusterRole             = "manifests/flannel-cluster-role.yaml"
	AssetPathFlannelClusterRoleBinding      = "manifests/flannel-cluster-role-binding.yaml"
	AssetPathFlannelSA                      = "manifests/flannel-sa.yaml"
	AssetPathCalico                         = "manifests/calico.yaml"
	AssetPathCalicoPolicyOnly               = "manifests/calico-policy-only.yaml"
	AssetPathCalicoCfg                      = "manifests/calico-config.yaml"
	AssetPathCalicoSA                       = "manifests/calico-service-account.yaml"
	AssetPathCalicoRole                     = "manifests/calico-role.yaml"
	AssetPathCalicoRoleBinding              = "manifests/calico-role-binding.yaml"
	AssetPathCalicoBGPConfigurationsCRD     = "manifests/calico-bgp-configurations-crd.yaml"
	AssetPathCalicoBGPPeersCRD              = "manifests/calico-bgp-peers-crd.yaml"
	AssetPathCalicoFelixConfigurationsCRD   = "manifests/calico-felix-configurations-crd.yaml"
	AssetPathCalicoGlobalNetworkPoliciesCRD = "manifests/calico-global-network-policies-crd.yaml"
	AssetPathCalicoNetworkPoliciesCRD       = "manifests/calico-network-policies-crd.yaml"
	AssetPathCalicoGlobalNetworkSetsCRD     = "manifests/calico-global-network-sets-crd.yaml"
	AssetPathCalicoIPPoolsCRD               = "manifests/calico-ip-pools-crd.yaml"
	AssetPathCalicoClusterInformationsCRD   = "manifests/calico-cluster-informations-crd.yaml"
	AssetPathAPIServerSecret                = "manifests/kube-apiserver-secret.yaml"
	AssetPathAPIServer                      = "manifests/kube-apiserver.yaml"
	AssetPathControllerManager              = "manifests/kube-controller-manager.yaml"
	AssetPathControllerManagerSA            = "manifests/kube-controller-manager-service-account.yaml"
	AssetPathControllerManagerRB            = "manifests/kube-controller-manager-role-binding.yaml"
	AssetPathControllerManagerSecret        = "manifests/kube-controller-manager-secret.yaml"
	AssetPathControllerManagerDisruption    = "manifests/kube-controller-manager-disruption.yaml"
	AssetPathScheduler                      = "manifests/kube-scheduler.yaml"
	AssetPathSchedulerDisruption            = "manifests/kube-scheduler-disruption.yaml"
	AssetPathCoreDNSClusterRoleBinding      = "manifests/coredns-cluster-role-binding.yaml"
	AssetPathCoreDNSClusterRole             = "manifests/coredns-cluster-role.yaml"
	AssetPathCoreDNSConfig                  = "manifests/coredns-config.yaml"
	AssetPathCoreDNSDeployment              = "manifests/coredns-deployment.yaml"
	AssetPathCoreDNSSA                      = "manifests/coredns-service-account.yaml"
	AssetPathCoreDNSSvc                     = "manifests/coredns-service.yaml"
	AssetPathSystemNamespace                = "manifests/kube-system-ns.yaml"
	AssetPathCheckpointer                   = "manifests/pod-checkpointer.yaml"
	AssetPathCheckpointerSA                 = "manifests/pod-checkpointer-sa.yaml"
	AssetPathCheckpointerRole               = "manifests/pod-checkpointer-role.yaml"
	AssetPathCheckpointerRoleBinding        = "manifests/pod-checkpointer-role-binding.yaml"
	AssetPathCheckpointerClusterRole        = "manifests/pod-checkpointer-cluster-role.yaml"
	AssetPathCheckpointerClusterRoleBinding = "manifests/pod-checkpointer-cluster-role-binding.yaml"
	AssetPathEtcdClientSecret               = "manifests/etcd-client-tls.yaml"
	AssetPathEtcdPeerSecret                 = "manifests/etcd-peer-tls.yaml"
	AssetPathEtcdServerSecret               = "manifests/etcd-server-tls.yaml"
	AssetPathCSRBootstrapRoleBinding        = "manifests/csr-bootstrap-role-binding.yaml"
	AssetPathCSRApproverRoleBinding         = "manifests/csr-approver-role-binding.yaml"
	AssetPathCSRRenewalRoleBinding          = "manifests/csr-renewal-role-binding.yaml"
	AssetPathKubeSystemSARoleBinding        = "manifests/kube-system-rbac-role-binding.yaml"
	AssetPathBootstrapManifests             = "bootstrap-manifests"
	AssetPathBootstrapAPIServer             = "bootstrap-manifests/bootstrap-apiserver.yaml"
	AssetPathBootstrapControllerManager     = "bootstrap-manifests/bootstrap-controller-manager.yaml"
	AssetPathBootstrapScheduler             = "bootstrap-manifests/bootstrap-scheduler.yaml"
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
	DNSServiceIP           net.IP
	CloudProvider          string
	NetworkProvider        string
	BootstrapSecretsSubdir string
	Images                 ImageVersions
}

// ImageVersions holds all the images (and their versions) that are rendered into the templates.
type ImageVersions struct {
	Etcd            string
	Flannel         string
	FlannelCNI      string
	Calico          string
	CalicoCNI       string
	CoreDNS         string
	Hyperkube       string
	Kenc            string
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
		etcdTLSAssets, err := newEtcdTLSAssets(conf.EtcdCACert, conf.EtcdClientCert, conf.EtcdClientKey, conf.CACert, conf.CAPrivKey, conf.EtcdServers)
		if err != nil {
			return Assets{}, err
		}
		as = append(as, etcdTLSAssets...)
	}

	kubeConfigAssets, err := newKubeConfigAssets(as, conf)
	if err != nil {
		return Assets{}, err
	}
	as = append(as, kubeConfigAssets...)

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

func (as Assets) WriteFiles(path string) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return err
	}
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return err
	}
	if len(files) > 0 {
		return errors.New("asset directory must be empty")
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
