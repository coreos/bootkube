package asset

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"text/template"

	"github.com/ghodss/yaml"

	"github.com/kubernetes-incubator/bootkube/pkg/asset/internal"
)

const (
	// The name of the k8s service that selects self-hosted etcd pods
	EtcdServiceName = "etcd-service"

	SecretEtcdPeer   = "etcd-peer-tls"
	SecretEtcdServer = "etcd-server-tls"
	SecretEtcdClient = "etcd-client-tls"

	NetworkFlannel = "flannel"
	NetworkCalico  = "experimental-calico"
	NetworkCanal   = "experimental-canal"

	secretNamespace     = "kube-system"
	secretAPIServerName = "kube-apiserver"
	secretCMName        = "kube-controller-manager"
)

type staticConfig struct {
	Images ImageVersions
}

func newStaticAssets(imageVersions ImageVersions) Assets {
	conf := staticConfig{Images: imageVersions}
	assets := Assets{
		MustCreateAssetFromTemplate(AssetPathScheduler, internal.SchedulerTemplate, conf),
		MustCreateAssetFromTemplate(AssetPathSchedulerDisruption, internal.SchedulerDisruptionTemplate, conf),
		MustCreateAssetFromTemplate(AssetPathControllerManagerDisruption, internal.ControllerManagerDisruptionTemplate, conf),
		MustCreateAssetFromTemplate(AssetPathCoreDNSClusterRoleBinding, internal.CoreDNSClusterRoleBindingTemplate, conf),
		MustCreateAssetFromTemplate(AssetPathCoreDNSClusterRole, internal.CoreDNSClusterRoleTemplate, conf),
		MustCreateAssetFromTemplate(AssetPathCoreDNSDeployment, internal.CoreDNSDeploymentTemplate, conf),
		MustCreateAssetFromTemplate(AssetPathCoreDNSSA, internal.CoreDNSServiceAccountTemplate, conf),
		MustCreateAssetFromTemplate(AssetPathCheckpointer, internal.CheckpointerTemplate, conf),
		MustCreateAssetFromTemplate(AssetPathCheckpointerSA, internal.CheckpointerServiceAccount, conf),
		MustCreateAssetFromTemplate(AssetPathCheckpointerRole, internal.CheckpointerRole, conf),
		MustCreateAssetFromTemplate(AssetPathCheckpointerRoleBinding, internal.CheckpointerRoleBinding, conf),
		MustCreateAssetFromTemplate(AssetPathCSRApproverRoleBinding, internal.CSRApproverRoleBindingTemplate, conf),
		MustCreateAssetFromTemplate(AssetPathCSRBootstrapRoleBinding, internal.CSRNodeBootstrapTemplate, conf),
		MustCreateAssetFromTemplate(AssetPathCSRRenewalRoleBinding, internal.CSRRenewalRoleBindingTemplate, conf),
		MustCreateAssetFromTemplate(AssetPathKubeSystemSARoleBinding, internal.KubeSystemSARoleBindingTemplate, conf),
	}
	return assets
}

func newDynamicAssets(conf Config) Assets {
	assets := Assets{
		MustCreateAssetFromTemplate(AssetPathControllerManager, internal.ControllerManagerTemplate, conf),
		MustCreateAssetFromTemplate(AssetPathControllerManagerSA, internal.ControllerManagerServiceAccount, conf),
		MustCreateAssetFromTemplate(AssetPathControllerManagerRB, internal.ControllerManagerClusterRoleBinding, conf),
		MustCreateAssetFromTemplate(AssetPathAPIServer, internal.APIServerTemplate, conf),
		MustCreateAssetFromTemplate(AssetPathProxy, internal.ProxyTemplate, conf),
		MustCreateAssetFromTemplate(AssetPathProxySA, internal.ProxyServiceAccount, conf),
		MustCreateAssetFromTemplate(AssetPathProxyRoleBinding, internal.ProxyClusterRoleBinding, conf),
		MustCreateAssetFromTemplate(AssetPathCoreDNSConfig, internal.CoreDNSConfigTemplate, conf),
		MustCreateAssetFromTemplate(AssetPathCoreDNSSvc, internal.CoreDNSSvcTemplate, conf),
		MustCreateAssetFromTemplate(AssetPathBootstrapAPIServer, internal.BootstrapAPIServerTemplate, conf),
		MustCreateAssetFromTemplate(AssetPathBootstrapControllerManager, internal.BootstrapControllerManagerTemplate, conf),
		MustCreateAssetFromTemplate(AssetPathBootstrapScheduler, internal.BootstrapSchedulerTemplate, conf),
	}
	switch conf.NetworkProvider {
	case NetworkFlannel:
		assets = append(assets,
			MustCreateAssetFromTemplate(AssetPathFlannel, internal.FlannelTemplate, conf),
			MustCreateAssetFromTemplate(AssetPathFlannelCfg, internal.FlannelCfgTemplate, conf),
			MustCreateAssetFromTemplate(AssetPathFlannelClusterRole, internal.FlannelClusterRole, conf),
			MustCreateAssetFromTemplate(AssetPathFlannelClusterRoleBinding, internal.FlannelClusterRoleBinding, conf),
			MustCreateAssetFromTemplate(AssetPathFlannelSA, internal.FlannelServiceAccount, conf),
		)
	case NetworkCalico:
		assets = append(assets,
			MustCreateAssetFromTemplate(AssetPathCalicoCfg, internal.CalicoCfgTemplate, conf),
			MustCreateAssetFromTemplate(AssetPathCalicoRole, internal.CalicoRoleTemplate, conf),
			MustCreateAssetFromTemplate(AssetPathCalicoRoleBinding, internal.CalicoRoleBindingTemplate, conf),
			MustCreateAssetFromTemplate(AssetPathCalicoSA, internal.CalicoServiceAccountTemplate, conf),
			MustCreateAssetFromTemplate(AssetPathCalico, internal.CalicoNodeTemplate, conf),
			MustCreateAssetFromTemplate(AssetPathCalicoBGPConfigurationsCRD, internal.CalicoBGPConfigurationsCRD, conf),
			MustCreateAssetFromTemplate(AssetPathCalicoBGPPeersCRD, internal.CalicoBGPPeersCRD, conf),
			MustCreateAssetFromTemplate(AssetPathCalicoFelixConfigurationsCRD, internal.CalicoFelixConfigurationsCRD, conf),
			MustCreateAssetFromTemplate(AssetPathCalicoGlobalNetworkPoliciesCRD, internal.CalicoGlobalNetworkPoliciesCRD, conf),
			MustCreateAssetFromTemplate(AssetPathCalicoGlobalNetworkSetsCRD, internal.CalicoGlobalNetworkSetsCRD, conf),
			MustCreateAssetFromTemplate(AssetPathCalicoNetworkPoliciesCRD, internal.CalicoNetworkPoliciesCRD, conf),
			MustCreateAssetFromTemplate(AssetPathCalicoClusterInformationsCRD, internal.CalicoClusterInformationsCRD, conf),
			MustCreateAssetFromTemplate(AssetPathCalicoIPPoolsCRD, internal.CalicoIPPoolsCRD, conf))
	case NetworkCanal:
		assets = append(assets,
			MustCreateAssetFromTemplate(AssetPathCalicoCfg, internal.CalicoCfgTemplate, conf),
			MustCreateAssetFromTemplate(AssetPathCalicoRole, internal.CalicoRoleTemplate, conf),
			MustCreateAssetFromTemplate(AssetPathCalicoRoleBinding, internal.CalicoRoleBindingTemplate, conf),
			MustCreateAssetFromTemplate(AssetPathCalicoSA, internal.CalicoServiceAccountTemplate, conf),
			MustCreateAssetFromTemplate(AssetPathCalicoPolicyOnly, internal.CalicoPolicyOnlyTemplate, conf),
			MustCreateAssetFromTemplate(AssetPathCalicoBGPConfigurationsCRD, internal.CalicoBGPConfigurationsCRD, conf),
			MustCreateAssetFromTemplate(AssetPathCalicoBGPPeersCRD, internal.CalicoBGPPeersCRD, conf),
			MustCreateAssetFromTemplate(AssetPathCalicoFelixConfigurationsCRD, internal.CalicoFelixConfigurationsCRD, conf),
			MustCreateAssetFromTemplate(AssetPathCalicoGlobalNetworkPoliciesCRD, internal.CalicoGlobalNetworkPoliciesCRD, conf),
			MustCreateAssetFromTemplate(AssetPathCalicoGlobalNetworkSetsCRD, internal.CalicoGlobalNetworkSetsCRD, conf),
			MustCreateAssetFromTemplate(AssetPathCalicoGlobalNetworkPoliciesCRD, internal.CalicoGlobalNetworkPoliciesCRD, conf),
			MustCreateAssetFromTemplate(AssetPathCalicoNetworkPoliciesCRD, internal.CalicoNetworkPoliciesCRD, conf),
			MustCreateAssetFromTemplate(AssetPathCalicoClusterInformationsCRD, internal.CalicoClusterInformationsCRD, conf),
			MustCreateAssetFromTemplate(AssetPathCalicoIPPoolsCRD, internal.CalicoIPPoolsCRD, conf))
	}
	return assets
}

const validBootstrapTokenChars = "0123456789abcdefghijklmnopqrstuvwxyz"

// newBootstrapToken constructs a bootstrap token in conformance with the following format:
// https://kubernetes.io/docs/admin/bootstrap-tokens/#token-format
func newBootstrapToken() (id string, secret string, err error) {
	// Read 6 random bytes for the id and 16 random bytes for the token (see spec for details).
	token := make([]byte, 6+16)
	if _, err := rand.Read(token); err != nil {
		return "", "", err
	}

	for i, b := range token {
		token[i] = validBootstrapTokenChars[int(b)%len(validBootstrapTokenChars)]
	}
	return string(token[:6]), string(token[6:]), nil
}

func newKubeConfigAssets(assets Assets, conf Config) ([]Asset, error) {
	caCert, err := assets.Get(AssetPathCACert)
	if err != nil {
		return nil, err
	}

	adminCert, err := assets.Get(AssetPathAdminCert)
	if err != nil {
		return nil, err
	}

	adminKey, err := assets.Get(AssetPathAdminKey)
	if err != nil {
		return nil, err
	}

	bootstrapTokenID, bootstrapTokenSecret, err := newBootstrapToken()
	if err != nil {
		return nil, err
	}

	cfg := struct {
		Server               string
		CACert               string
		AdminCert            string
		AdminKey             string
		BootstrapTokenID     string
		BootstrapTokenSecret string
	}{
		Server:               conf.APIServers[0].String(),
		CACert:               base64.StdEncoding.EncodeToString(caCert.Data),
		AdminCert:            base64.StdEncoding.EncodeToString(adminCert.Data),
		AdminKey:             base64.StdEncoding.EncodeToString(adminKey.Data),
		BootstrapTokenID:     bootstrapTokenID,
		BootstrapTokenSecret: bootstrapTokenSecret,
	}

	templates := []struct {
		path string
		tmpl []byte
	}{
		{AssetPathAdminKubeConfig, internal.AdminKubeConfigTemplate},
		{AssetPathKubeConfigInCluster, internal.KubeConfigInClusterTemplate},
		{AssetPathKubeletKubeConfig, internal.KubeletKubeConfigTemplate},
		{AssetPathKubeletBootstrapToken, internal.KubeletBootstrappingToken},
	}

	var as []Asset
	for _, t := range templates {
		a, err := assetFromTemplate(t.path, t.tmpl, cfg)
		if err != nil {
			return nil, fmt.Errorf("rendering template %s: %v", t.path, err)
		}
		as = append(as, a)
	}
	return as, nil
}

func newAPIServerSecretAsset(assets Assets, etcdUseTLS bool) (Asset, error) {
	secretAssets := []string{
		AssetPathAPIServerKey,
		AssetPathAPIServerCert,
		AssetPathServiceAccountPubKey,
		AssetPathCACert,
	}
	if etcdUseTLS {
		secretAssets = append(secretAssets, []string{
			AssetPathEtcdClientCA,
			AssetPathEtcdClientCert,
			AssetPathEtcdClientKey,
		}...)
	}

	secretYAML, err := secretFromAssets(secretAPIServerName, secretNamespace, secretAssets, assets)
	if err != nil {
		return Asset{}, err
	}

	return Asset{Name: AssetPathAPIServerSecret, Data: secretYAML}, nil
}

func newControllerManagerSecretAsset(assets Assets) (Asset, error) {
	secretAssets := []string{
		AssetPathServiceAccountPrivKey,
		AssetPathCACert,
		AssetPathCAKey,
	}

	secretYAML, err := secretFromAssets(secretCMName, secretNamespace, secretAssets, assets)
	if err != nil {
		return Asset{}, err
	}

	return Asset{Name: AssetPathControllerManagerSecret, Data: secretYAML}, nil
}

// TODO(aaron): use actual secret object (need to wrap in apiversion/type)
type secret struct {
	ApiVersion string            `json:"apiVersion"`
	Kind       string            `json:"kind"`
	Metadata   map[string]string `json:"metadata"`
	Type       string            `json:"type"`
	Data       map[string]string `json:"data"`
}

func secretFromAssets(name, namespace string, assetNames []string, assets Assets) ([]byte, error) {
	data := make(map[string]string)
	for _, an := range assetNames {
		a, err := assets.Get(an)
		if err != nil {
			return []byte{}, err
		}
		data[filepath.Base(a.Name)] = base64.StdEncoding.EncodeToString(a.Data)
	}
	return yaml.Marshal(secret{
		ApiVersion: "v1",
		Kind:       "Secret",
		Type:       "Opaque",
		Metadata: map[string]string{
			"name":      name,
			"namespace": namespace,
		},
		Data: data,
	})
}

func MustCreateAssetFromTemplate(name string, template []byte, data interface{}) Asset {
	a, err := assetFromTemplate(name, template, data)
	if err != nil {
		panic(err)
	}
	return a
}

func assetFromTemplate(name string, tb []byte, data interface{}) (Asset, error) {
	tmpl, err := template.New(name).Parse(string(tb))
	if err != nil {
		return Asset{}, err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return Asset{}, err
	}
	return Asset{Name: name, Data: buf.Bytes()}, nil
}
