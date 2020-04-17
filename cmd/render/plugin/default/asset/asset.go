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
	"reflect"
	"strings"

	"github.com/kubernetes-sigs/bootkube/pkg/tlsutil"
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
	AssetPathAggregatorCA                   = "tls/front-proxy-ca.crt"
	AssetPathFrontProxyClientCert           = "tls/front-proxy-client.crt"
	AssetPathFrontProxyClientKey            = "tls/front-proxy-client.key"
	AssetPathServiceAccountPrivKey          = "tls/service-account.key"
	AssetPathServiceAccountPubKey           = "tls/service-account.pub"
	AssetPathKubeletClientCert              = "tls/apiserver-kubelet-client.crt"
	AssetPathKubeletClientKey               = "tls/apiserver-kubelet-client.key"
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
	AssetPathCharts                         = "charts"
)

var BootstrapSecretsDir = "/etc/kubernetes/bootstrap-secrets" // Overridden for testing.

// AssetConfig holds all configuration needed when generating
// the default set of assets.
type Config struct {
	ClusterName            string
	EtcdCACert             *x509.Certificate
	EtcdClientCert         *x509.Certificate
	EtcdClientKey          *rsa.PrivateKey
	EtcdServers            []*url.URL
	EtcdUseTLS             bool
	APIServers             []*url.URL
	CACert                 *x509.Certificate
	CAPrivKey              *rsa.PrivateKey
	AltNames               *tlsutil.AltNames
	PodCIDRs               []*net.IPNet
	ServiceCIDRs           []*net.IPNet
	APIServiceIPs          []net.IP
	DNSServiceIPs          []net.IP
	CloudProvider          string
	NetworkProvider        string
	BootstrapSecretsSubdir string
	Images                 ImageVersions

	// PodCIDR describes the networking subnet to be used for inter-pod networking.
	//
	// Deprecated: PodCIDR exists only for compatibility with older external
	// systems.  Please use PodCIDRs instead, which allows for dual-stack
	// configurations.
	PodCIDR *net.IPNet

	// ServiceCIDR describes the networking subnet to be used to expose services.
	//
	// Deprecated: ServiceCIDR exists only for compatibility with older external
	// systems.  Please use ServiceCIDRs instead, which allows for dual-stack
	// configurations.  If both are specified, only ServiceCIDRs will be used.
	ServiceCIDR *net.IPNet

	// APIServiceIP describes the in-cluster IP address by which the API Servers may be reached.
	//
	// Deprecated: APIServiceIP exists only for compatibility with older
	// external systems.  Please use APIServiceIPs instead, which allows for
	// dual-stack configurations.  If both are specified, only APIServiceIPs
	// will be used.
	APIServiceIP net.IP

	// DNSServiceIP describes the in-cluster IP address by which the cluster DNS servers may be reached.
	//
	// Deprecated:  DNSServiceIP exists only for compatibility with older
	// external systems.  Please use DNSServiceIPs instead, which allows for
	// dual-stack configurations.  If both are specified, only DNSServiceIPs
	// will be used.
	DNSServiceIP net.IP
}

// BindAllAddress indicates the address to use when binding all IPs.  If this
// is an IPv6 or dual-stack system, `::` will be returned.  Otherwise
// `0.0.0.0`.
func (c Config) BindAllAddress() string {
	if containsNonLocalIPv6(c.APIServiceIPs) {
		return "::"
	}
	return "0.0.0.0"
}

// ServiceCIDRsString returns a "," concatenated string for the ServiceCIDRs
func (c Config) ServiceCIDRsString() string {
	return joinStringsFromSliceOrSingle(stringerSlice(c.ServiceCIDRs), c.ServiceCIDR)
}

// PodCIDRsString returns a "," concatenated string for the PodCIDRs
func (c Config) PodCIDRsString() string {
	return joinStringsFromSliceOrSingle(stringerSlice(c.PodCIDRs), c.PodCIDR)
}

// APIServiceIPsString returns a "," concatenated string for the APIServiceIPs
func (c Config) APIServiceIPsString() string {
	return joinStringsFromSliceOrSingle(stringerSlice(c.APIServiceIPs), c.APIServiceIP)
}

// DNSServiceIPsString returns a "," concatenated string for the DNSServiceIPs
func (c Config) DNSServiceIPsString() string {
	return joinStringsFromSliceOrSingle(stringerSlice(c.DNSServiceIPs), c.DNSServiceIP)
}

func stringerSlice(in interface{}) []string {
	var ok bool

	type Stringer interface {
		String() string
	}
	var stringer Stringer

	if in == nil {
		return nil
	}

	r := reflect.ValueOf(in)
	if r.Len() == 0 {
		return nil
	}

	rval := make([]string, r.Len())
	for i := 0; i < r.Len(); i++ {
		stringer, ok = r.Index(i).Interface().(fmt.Stringer)
		if !ok {
			rval[i] = ""
			continue
		}
		rval[i] = stringer.String()
	}
	return rval
}

func joinStringsFromSliceOrSingle(inSlice []string, inSingle fmt.Stringer) string {
	if len(inSlice) > 0 {
		return strings.Join(inSlice, ",")
	}

	if inSingle == nil {
		return ""
	}

	return inSingle.String()
}

func containsNonLocalIPv6(in []net.IP) bool {
	for _, ip := range in {
		if ip == nil || ip.IsLoopback() || ip.IsUnspecified() {
			continue
		}

		if ip.To4() == nil && ip.To16() != nil {
			return true
		}
	}
	return false
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
	if len(conf.APIServiceIPs) > 0 {
		conf.AltNames.IPs = append(conf.AltNames.IPs, conf.APIServiceIPs...)
	} else {
		conf.AltNames.IPs = append(conf.AltNames.IPs, conf.APIServiceIP)
	}

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
