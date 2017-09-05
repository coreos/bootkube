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

	"github.com/kubernetes-incubator/bootkube/pkg/asset/internal"
	"github.com/kubernetes-incubator/bootkube/pkg/tlsutil"
)

const (
	AssetPathSecrets                     = "tls"
	AssetPathCAKey                       = "tls/ca.key"
	AssetPathCACert                      = "tls/ca.crt"
	AssetPathAPIServerKey                = "tls/apiserver.key"
	AssetPathAPIServerCert               = "tls/apiserver.crt"
	AssetPathEtcdClientCA                = "tls/etcd-client-ca.crt"
	AssetPathEtcdClientCert              = "tls/etcd-client.crt"
	AssetPathEtcdClientKey               = "tls/etcd-client.key"
	AssetPathEtcdServerCA                = "tls/etcd/server-ca.crt"
	AssetPathEtcdServerCert              = "tls/etcd/server.crt"
	AssetPathEtcdServerKey               = "tls/etcd/server.key"
	AssetPathEtcdPeerCA                  = "tls/etcd/peer-ca.crt"
	AssetPathEtcdPeerCert                = "tls/etcd/peer.crt"
	AssetPathEtcdPeerKey                 = "tls/etcd/peer.key"
	AssetPathServiceAccountPrivKey       = "tls/service-account.key"
	AssetPathServiceAccountPubKey        = "tls/service-account.pub"
	AssetPathKubeletKey                  = "tls/kubelet.key"
	AssetPathKubeletCert                 = "tls/kubelet.crt"
	AssetPathKubeConfig                  = "auth/kubeconfig"
	AssetPathManifests                   = "manifests"
	AssetPathKubelet                     = "manifests/kubelet.yaml"
	AssetPathProxy                       = "manifests/kube-proxy.yaml"
	AssetPathKubeFlannel                 = "manifests/kube-flannel.yaml"
	AssetPathKubeFlannelCfg              = "manifests/kube-flannel-cfg.yaml"
	AssetPathCalico                      = "manifests/calico.yaml"
	AssetPathCalicoCfg                   = "manifests/calico-config.yaml"
	AssetPathCalcioSA                    = "manifests/calico-service-account.yaml"
	AssetPathCalcioRole                  = "manifests/calico-role.yaml"
	AssetPathCalcioRoleBinding           = "manifests/calico-role-binding.yaml"
	AssetPathCalicoBGPConfigsCRD         = "manifests/calico-bgp-configs-crd.yaml"
	AssetPathCalicoFelixConfigsCRD       = "manifests/calico-felix-configs-crd.yaml"
	AssetPathCalicoNetworkPoliciesCRD    = "manifests/calico-network-policies-crd.yaml"
	AssetPathCalicoIPPoolsCRD            = "manifests/calico-ip-pools-crd.yaml"
	AssetPathAPIServerSecret             = "manifests/kube-apiserver-secret.yaml"
	AssetPathAPIServer                   = "manifests/kube-apiserver.yaml"
	AssetPathControllerManager           = "manifests/kube-controller-manager.yaml"
	AssetPathControllerManagerSecret     = "manifests/kube-controller-manager-secret.yaml"
	AssetPathControllerManagerDisruption = "manifests/kube-controller-manager-disruption.yaml"
	AssetPathScheduler                   = "manifests/kube-scheduler.yaml"
	AssetPathSchedulerDisruption         = "manifests/kube-scheduler-disruption.yaml"
	AssetPathKubeDNSDeployment           = "manifests/kube-dns-deployment.yaml"
	AssetPathKubeDNSSvc                  = "manifests/kube-dns-svc.yaml"
	AssetPathSystemNamespace             = "manifests/kube-system-ns.yaml"
	AssetPathCheckpointer                = "manifests/pod-checkpointer.yaml"
	AssetPathEtcdOperator                = "manifests/etcd-operator.yaml"
	AssetPathEtcdSvc                     = "manifests/etcd-service.yaml"
	AssetPathEtcdClientSecret            = "manifests/etcd-client-tls.yaml"
	AssetPathEtcdPeerSecret              = "manifests/etcd-peer-tls.yaml"
	AssetPathEtcdServerSecret            = "manifests/etcd-server-tls.yaml"
	AssetPathKenc                        = "manifests/kube-etcd-network-checkpointer.yaml"
	AssetPathKubeSystemSARoleBinding     = "manifests/kube-system-rbac-role-binding.yaml"
	AssetPathBootstrapManifests          = "bootstrap-manifests"
	AssetPathBootstrapAPIServer          = "bootstrap-manifests/bootstrap-apiserver.yaml"
	AssetPathBootstrapControllerManager  = "bootstrap-manifests/bootstrap-controller-manager.yaml"
	AssetPathBootstrapScheduler          = "bootstrap-manifests/bootstrap-scheduler.yaml"
	AssetPathBootstrapEtcd               = "bootstrap-manifests/bootstrap-etcd.yaml"
	AssetPathBootstrapEtcdService        = "etcd/bootstrap-etcd-service.json"
	AssetPathMigrateEtcdCluster          = "etcd/migrate-etcd-cluster.json"
)

const (
	TemplatePathKubeConfig                  = "auth/kubeconfig"
	TemplatePathKubeSystemSARoleBinding     = "manifests/kube-system-rbac-role-binding.yaml"
	TemplatePathKubelet                     = "manifests/kubelet.yaml"
	TemplatePathAPIServer                   = "manifests/kube-apiserver.yaml"
	TemplatePathBootstrapAPIServer          = "bootstrap-manifests/bootstrap-apiserver.yaml"
	TemplatePathKenc                        = "manifests/kube-etcd-network-checkpointer.yaml"
	TemplatePathCheckpointer                = "manifests/pod-checkpointer.yaml"
	TemplatePathControllerManager           = "manifests/kube-controller-manager.yaml"
	TemplatePathBootstrapControllerManager  = "bootstrap-manifests/bootstrap-controller-manager.yaml"
	TemplatePathControllerManagerDisruption = "manifests/kube-controller-manager-disruption.yaml"
	TemplatePathScheduler                   = "manifests/kube-scheduler.yaml"
	TemplatePathBootstrapScheduler          = "bootstrap-manifests/bootstrap-scheduler.yaml"
	TemplatePathSchedulerDisruption         = "manifests/kube-scheduler-disruption.yaml"
	TemplatePathProxy                       = "manifests/kube-proxy.yaml"
	TemplatePathDNSDeployment               = "manifests/kube-dns-deployment.yaml"
	TemplatePathDNSSvc                      = "manifests/kube-dns-svc.yaml"
	TemplatePathEtcdOperator                = "manifests/etcd-operator.yaml"
	TemplatePathEtcdSvc                     = "manifests/etcd-service.yaml"
	TemplatePathBootstrapEtcd               = "bootstrap-manifests/bootstrap-etcd.yaml"
	TemplatePathBootstrapEtcdSvc            = "etcd/bootstrap-etcd-service.json"
	TemplatePathEtcdCRD                     = "etcd/migrate-etcd-cluster.json"
	TemplatePathKubeFlannelCfg              = "manifests/kube-flannel.yaml"
	TemplatePathKubeFlannel                 = "manifests/kube-flannel-cfg.yaml"
	TemplatePathCalico                      = "manifests/calico.yaml"
	TemplatePathCalicoCfg                   = "manifests/calico-config.yaml"
	TemplatePathCalcioSA                    = "manifests/calico-service-account.yaml"
	TemplatePathCalcioRole                  = "manifests/calico-role.yaml"
	TemplatePathCalcioRoleBinding           = "manifests/calico-role-binding.yaml"
	TemplatePathCalicoBGPConfigsCRD         = "manifests/calico-bgp-configs-crd.yaml"
	TemplatePathCalicoFelixConfigsCRD       = "manifests/calico-felix-configs-crd.yaml"
	TemplatePathCalicoNetworkPoliciesCRD    = "manifests/calico-network-policies-crd.yaml"
	TemplatePathCalicoIPPoolsCRD            = "manifests/calico-ip-pools-crd.yaml"
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

type TemplateContent struct {
	KubeConfigTemplate                  []byte
	KubeSystemSARoleBindingTemplate     []byte
	KubeletTemplate                     []byte
	APIServerTemplate                   []byte
	BootstrapAPIServerTemplate          []byte
	KencTemplate                        []byte
	CheckpointerTemplate                []byte
	ControllerManagerTemplate           []byte
	BootstrapControllerManagerTemplate  []byte
	ControllerManagerDisruptionTemplate []byte
	SchedulerTemplate                   []byte
	BootstrapSchedulerTemplate          []byte
	SchedulerDisruptionTemplate         []byte
	ProxyTemplate                       []byte
	DNSDeploymentTemplate               []byte
	DNSSvcTemplate                      []byte
	EtcdOperatorTemplate                []byte
	EtcdSvcTemplate                     []byte
	BootstrapEtcdTemplate               []byte
	BootstrapEtcdSvcTemplate            []byte
	EtcdCRDTemplate                     []byte
	KubeFlannelCfgTemplate              []byte
	KubeFlannelTemplate                 []byte
	CalicoCfgTemplate                   []byte
	CalicoRoleTemplate                  []byte
	CalicoRoleBindingTemplate           []byte
	CalicoServiceAccountTemplate        []byte
	CalicoNodeTemplate                  []byte
	CalicoBGPConfigsCRD                 []byte
	CalicoFelixConfigsCRD               []byte
	CalicoNetworkPoliciesCRD            []byte
	CalicoIPPoolsCRD                    []byte
}

func NewTemplateContent(path string) (*TemplateContent, error) {
	if path == "" {
		return defaultInternalTemplateContent(), nil
	}
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	return readTemplateContentFromFiles(path)
}

func readTemplateContentFromFiles(path string) (*TemplateContent, error) {
	tr := newTemplateReader(path)
	templates := &TemplateContent{
		KubeConfigTemplate:                  tr.Read(TemplatePathKubeConfig),
		KubeSystemSARoleBindingTemplate:     tr.Read(TemplatePathKubeSystemSARoleBinding),
		KubeletTemplate:                     tr.Read(TemplatePathKubelet),
		APIServerTemplate:                   tr.Read(TemplatePathAPIServer),
		BootstrapAPIServerTemplate:          tr.Read(TemplatePathBootstrapAPIServer),
		KencTemplate:                        tr.Read(TemplatePathKenc),
		CheckpointerTemplate:                tr.Read(TemplatePathCheckpointer),
		ControllerManagerTemplate:           tr.Read(TemplatePathControllerManager),
		BootstrapControllerManagerTemplate:  tr.Read(TemplatePathBootstrapControllerManager),
		ControllerManagerDisruptionTemplate: tr.Read(TemplatePathControllerManagerDisruption),
		SchedulerTemplate:                   tr.Read(TemplatePathScheduler),
		BootstrapSchedulerTemplate:          tr.Read(TemplatePathBootstrapScheduler),
		SchedulerDisruptionTemplate:         tr.Read(TemplatePathSchedulerDisruption),
		ProxyTemplate:                       tr.Read(TemplatePathProxy),
		DNSDeploymentTemplate:               tr.Read(TemplatePathDNSDeployment),
		DNSSvcTemplate:                      tr.Read(TemplatePathDNSSvc),
		EtcdOperatorTemplate:                tr.Read(TemplatePathEtcdOperator),
		EtcdSvcTemplate:                     tr.Read(TemplatePathEtcdSvc),
		BootstrapEtcdTemplate:               tr.Read(TemplatePathBootstrapEtcd),
		BootstrapEtcdSvcTemplate:            tr.Read(TemplatePathBootstrapEtcdSvc),
		EtcdCRDTemplate:                     tr.Read(TemplatePathEtcdCRD),
		KubeFlannelCfgTemplate:              tr.Read(TemplatePathKubeFlannelCfg),
		KubeFlannelTemplate:                 tr.Read(TemplatePathKubeFlannel),
		CalicoCfgTemplate:                   tr.Read(TemplatePathCalicoCfg),
		CalicoRoleTemplate:                  tr.Read(TemplatePathCalcioRole),
		CalicoRoleBindingTemplate:           tr.Read(TemplatePathCalcioRoleBinding),
		CalicoServiceAccountTemplate:        tr.Read(TemplatePathCalcioSA),
		CalicoNodeTemplate:                  tr.Read(TemplatePathCalico),
		CalicoBGPConfigsCRD:                 tr.Read(TemplatePathCalicoBGPConfigsCRD),
		CalicoFelixConfigsCRD:               tr.Read(TemplatePathCalicoFelixConfigsCRD),
		CalicoNetworkPoliciesCRD:            tr.Read(TemplatePathCalicoNetworkPoliciesCRD),
		CalicoIPPoolsCRD:                    tr.Read(TemplatePathCalicoIPPoolsCRD),
	}

	return templates, tr.Error()
}

type templateReader struct {
	basePath string
	err      error
}

func newTemplateReader(basePath string) *templateReader {
	return &templateReader{
		basePath: basePath,
	}
}

func (tr *templateReader) Read(templateLocation string) []byte {
	if tr.err != nil {
		return nil
	}

	f, err := os.Open(path.Join(tr.basePath, templateLocation))
	if err != nil {
		tr.err = err
		return nil
	}
	defer f.Close()
	data, err := ioutil.ReadAll(f)
	tr.err = err
	return data
}

func (tr *templateReader) Error() error {
	return tr.err
}

func defaultInternalTemplateContent() *TemplateContent {
	return &TemplateContent{
		KubeConfigTemplate:                  internal.KubeConfigTemplate,
		KubeSystemSARoleBindingTemplate:     internal.KubeSystemSARoleBindingTemplate,
		KubeletTemplate:                     internal.KubeletTemplate,
		APIServerTemplate:                   internal.APIServerTemplate,
		BootstrapAPIServerTemplate:          internal.BootstrapAPIServerTemplate,
		KencTemplate:                        internal.KencTemplate,
		CheckpointerTemplate:                internal.CheckpointerTemplate,
		ControllerManagerTemplate:           internal.ControllerManagerTemplate,
		BootstrapControllerManagerTemplate:  internal.BootstrapControllerManagerTemplate,
		ControllerManagerDisruptionTemplate: internal.ControllerManagerDisruptionTemplate,
		SchedulerTemplate:                   internal.SchedulerTemplate,
		BootstrapSchedulerTemplate:          internal.BootstrapSchedulerTemplate,
		SchedulerDisruptionTemplate:         internal.SchedulerDisruptionTemplate,
		ProxyTemplate:                       internal.ProxyTemplate,
		DNSDeploymentTemplate:               internal.DNSDeploymentTemplate,
		DNSSvcTemplate:                      internal.DNSSvcTemplate,
		EtcdOperatorTemplate:                internal.EtcdOperatorTemplate,
		EtcdSvcTemplate:                     internal.EtcdSvcTemplate,
		BootstrapEtcdTemplate:               internal.BootstrapEtcdTemplate,
		BootstrapEtcdSvcTemplate:            internal.BootstrapEtcdSvcTemplate,
		EtcdCRDTemplate:                     internal.EtcdCRDTemplate,
		KubeFlannelCfgTemplate:              internal.KubeFlannelCfgTemplate,
		KubeFlannelTemplate:                 internal.KubeFlannelTemplate,
		CalicoCfgTemplate:                   internal.CalicoCfgTemplate,
		CalicoRoleTemplate:                  internal.CalicoRoleTemplate,
		CalicoRoleBindingTemplate:           internal.CalicoRoleBindingTemplate,
		CalicoServiceAccountTemplate:        internal.CalicoServiceAccountTemplate,
		CalicoNodeTemplate:                  internal.CalicoNodeTemplate,
		CalicoBGPConfigsCRD:                 internal.CalicoBGPConfigsCRD,
		CalicoFelixConfigsCRD:               internal.CalicoFelixConfigsCRD,
		CalicoNetworkPoliciesCRD:            internal.CalicoNetworkPoliciesCRD,
		CalicoIPPoolsCRD:                    internal.CalicoIPPoolsCRD,
	}
}

// NewDefaultAssets returns a list of default assets, optionally
// configured via a user provided AssetConfig. Default assets include
// TLS assets (certs, keys and secrets), and k8s component manifests.
func NewDefaultAssets(templates *TemplateContent, conf Config) (Assets, error) {
	conf.BootstrapSecretsSubdir = path.Base(BootstrapSecretsDir)

	as := newStaticAssets(templates, conf.Images)
	as = append(as, newDynamicAssets(templates, conf)...)

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
	kubeConfig, err := newKubeConfigAsset(templates, as, conf)
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
