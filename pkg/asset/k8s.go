package asset

import (
	"bytes"
	"encoding/base64"
	"path/filepath"
	"text/template"

	"github.com/ghodss/yaml"
)

const (
	// The name of the k8s service that selects self-hosted etcd pods
	EtcdServiceName = "etcd-service"

	SecretEtcdPeer   = "etcd-peer-tls"
	SecretEtcdServer = "etcd-server-tls"
	SecretEtcdClient = "etcd-client-tls"

	secretNamespace     = "kube-system"
	secretAPIServerName = "kube-apiserver"
	secretCMName        = "kube-controller-manager"
)

type staticConfig struct {
	Images ImageVersions
}

func newStaticAssets(templates *TemplateContent, imageVersions ImageVersions) Assets {
	conf := staticConfig{Images: imageVersions}
	assets := Assets{
		MustCreateAssetFromTemplate(AssetPathScheduler, templates.SchedulerTemplate, conf),
		MustCreateAssetFromTemplate(AssetPathSchedulerDisruption, templates.SchedulerDisruptionTemplate, conf),
		MustCreateAssetFromTemplate(AssetPathControllerManagerDisruption, templates.ControllerManagerDisruptionTemplate, conf),
		MustCreateAssetFromTemplate(AssetPathKubeDNSDeployment, templates.DNSDeploymentTemplate, conf),
		MustCreateAssetFromTemplate(AssetPathCheckpointer, templates.CheckpointerTemplate, conf),
		MustCreateAssetFromTemplate(AssetPathKubeSystemSARoleBinding, templates.KubeSystemSARoleBindingTemplate, conf),
	}
	return assets
}

func newDynamicAssets(templates *TemplateContent, conf Config) Assets {
	assets := Assets{
		MustCreateAssetFromTemplate(AssetPathControllerManager, templates.ControllerManagerTemplate, conf),
		MustCreateAssetFromTemplate(AssetPathAPIServer, templates.APIServerTemplate, conf),
		MustCreateAssetFromTemplate(AssetPathProxy, templates.ProxyTemplate, conf),
		MustCreateAssetFromTemplate(AssetPathKubeFlannelCfg, templates.KubeFlannelCfgTemplate, conf),
		MustCreateAssetFromTemplate(AssetPathKubeFlannel, templates.KubeFlannelTemplate, conf),
		MustCreateAssetFromTemplate(AssetPathKubeDNSSvc, templates.DNSSvcTemplate, conf),
		MustCreateAssetFromTemplate(AssetPathBootstrapAPIServer, templates.BootstrapAPIServerTemplate, conf),
		MustCreateAssetFromTemplate(AssetPathBootstrapControllerManager, templates.BootstrapControllerManagerTemplate, conf),
		MustCreateAssetFromTemplate(AssetPathBootstrapScheduler, templates.BootstrapSchedulerTemplate, conf),
	}
	if conf.SelfHostKubelet {
		assets = append(assets, MustCreateAssetFromTemplate(AssetPathKubelet, templates.KubeletTemplate, conf))
	}
	if conf.SelfHostedEtcd {
		conf.EtcdServiceName = EtcdServiceName
		assets = append(assets,
			MustCreateAssetFromTemplate(AssetPathEtcdOperator, templates.EtcdOperatorTemplate, conf),
			MustCreateAssetFromTemplate(AssetPathEtcdSvc, templates.EtcdSvcTemplate, conf),
			MustCreateAssetFromTemplate(AssetPathKenc, templates.KencTemplate, conf),
			MustCreateAssetFromTemplate(AssetPathBootstrapEtcd, templates.BootstrapEtcdTemplate, conf),
			MustCreateAssetFromTemplate(AssetPathBootstrapEtcdService, templates.BootstrapEtcdSvcTemplate, conf),
			MustCreateAssetFromTemplate(AssetPathMigrateEtcdCluster, templates.EtcdCRDTemplate, conf))
	}
	if conf.CalicoNetworkPolicy {
		assets = append(assets,
			MustCreateAssetFromTemplate(AssetPathCalicoCfg, templates.CalicoCfgTemplate, conf),
			MustCreateAssetFromTemplate(AssetPathCalcioRole, templates.CalicoRoleTemplate, conf),
			MustCreateAssetFromTemplate(AssetPathCalcioRoleBinding, templates.CalicoRoleBindingTemplate, conf),
			MustCreateAssetFromTemplate(AssetPathCalcioSA, templates.CalicoServiceAccountTemplate, conf),
			MustCreateAssetFromTemplate(AssetPathCalico, templates.CalicoNodeTemplate, conf),
			MustCreateAssetFromTemplate(AssetPathCalicoBGPConfigsCRD, templates.CalicoBGPConfigsCRD, conf),
			MustCreateAssetFromTemplate(AssetPathCalicoFelixConfigsCRD, templates.CalicoFelixConfigsCRD, conf),
			MustCreateAssetFromTemplate(AssetPathCalicoNetworkPoliciesCRD, templates.CalicoNetworkPoliciesCRD, conf),
			MustCreateAssetFromTemplate(AssetPathCalicoIPPoolsCRD, templates.CalicoIPPoolsCRD, conf))
	}
	return assets
}

func newKubeConfigAsset(templates *TemplateContent, assets Assets, conf Config) (Asset, error) {
	caCert, err := assets.Get(AssetPathCACert)
	if err != nil {
		return Asset{}, err
	}

	kubeletCert, err := assets.Get(AssetPathKubeletCert)
	if err != nil {
		return Asset{}, err
	}

	kubeletKey, err := assets.Get(AssetPathKubeletKey)
	if err != nil {
		return Asset{}, err
	}

	type templateCfg struct {
		Server      string
		CACert      string
		KubeletCert string
		KubeletKey  string
	}

	return assetFromTemplate(AssetPathKubeConfig, templates.KubeConfigTemplate, templateCfg{
		Server:      conf.APIServers[0].String(),
		CACert:      base64.StdEncoding.EncodeToString(caCert.Data),
		KubeletCert: base64.StdEncoding.EncodeToString(kubeletCert.Data),
		KubeletKey:  base64.StdEncoding.EncodeToString(kubeletKey.Data),
	})
}

func newSelfHostedEtcdSecretAssets(assets Assets) (Assets, error) {
	var res Assets

	secretYAML, err := secretFromAssets(SecretEtcdPeer, secretNamespace, []string{
		AssetPathEtcdPeerCA,
		AssetPathEtcdPeerCert,
		AssetPathEtcdPeerKey,
	}, assets)
	if err != nil {
		return nil, err
	}
	res = append(res, Asset{Name: AssetPathEtcdPeerSecret, Data: secretYAML})

	secretYAML, err = secretFromAssets(SecretEtcdServer, secretNamespace, []string{
		AssetPathEtcdServerCA,
		AssetPathEtcdServerCert,
		AssetPathEtcdServerKey,
	}, assets)
	if err != nil {
		return nil, err
	}
	res = append(res, Asset{Name: AssetPathEtcdServerSecret, Data: secretYAML})

	secretYAML, err = secretFromAssets(SecretEtcdClient, secretNamespace, []string{
		AssetPathEtcdClientCA,
		AssetPathEtcdClientCert,
		AssetPathEtcdClientKey,
	}, assets)
	if err != nil {
		return nil, err
	}
	res = append(res, Asset{Name: AssetPathEtcdClientSecret, Data: secretYAML})

	return res, nil
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
		AssetPathCACert, //TODO(aaron): do we want this also distributed as secret? or expect available on host?
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
