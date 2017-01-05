package asset

import (
	"bytes"
	"encoding/base64"
	"path/filepath"
	"text/template"

	"github.com/ghodss/yaml"

	"github.com/kubernetes-incubator/bootkube/pkg/asset/internal"
)

const (
	secretNamespace     = "kube-system"
	secretAPIServerName = "kube-apiserver"
	secretCMName        = "kube-controller-manager"
)

func newStaticAssets(selfHostKubelet, selfHostedEtcd bool) Assets {
	var noData interface{}
	assets := Assets{
		mustCreateAssetFromTemplate(AssetPathScheduler, internal.SchedulerTemplate, noData),
		mustCreateAssetFromTemplate(AssetPathSchedulerDisruption, internal.SchedulerDisruptionTemplate, noData),
		mustCreateAssetFromTemplate(AssetPathControllerManagerDisruption, internal.ControllerManagerDisruptionTemplate, noData),
		mustCreateAssetFromTemplate(AssetPathProxy, internal.ProxyTemplate, noData),
		mustCreateAssetFromTemplate(AssetPathKubeDNSDeployment, internal.DNSDeploymentTemplate, noData),
		mustCreateAssetFromTemplate(AssetPathKubeDNSSvc, internal.DNSSvcTemplate, noData),
		mustCreateAssetFromTemplate(AssetPathCheckpointer, internal.CheckpointerTemplate, noData),
		mustCreateAssetFromTemplate(AssetPathKubeFlannel, internal.KubeFlannelTemplate, noData),
		mustCreateAssetFromTemplate(AssetPathKubeFlannelCfg, internal.KubeFlannelCfgTemplate, noData),
		mustCreateAssetFromTemplate(AssetPathKubeletBootstrapRoleBinding, internal.KubeletBootstrapRoleBindingTemplate, noData),
		mustCreateAssetFromTemplate(AssetPathKubeSystemSARoleBinding, internal.KubeSystemSARoleBindingTemplate, noData),
	}
	if selfHostKubelet {
		assets = append(assets, mustCreateAssetFromTemplate(AssetPathKubelet, internal.KubeletTemplate, noData))
	}
	if selfHostedEtcd {
		assets = append(assets,
			mustCreateAssetFromTemplate(AssetPathEtcdOperator, internal.EtcdOperatorTemplate, noData),
			mustCreateAssetFromTemplate(AssetPathEtcdSvc, internal.EtcdSvcTemplate, noData),
		)
	}

	return assets
}

func newDynamicAssets(conf Config) Assets {
	return Assets{
		mustCreateAssetFromTemplate(AssetPathControllerManager, internal.ControllerManagerTemplate, conf),
		mustCreateAssetFromTemplate(AssetPathAPIServer, internal.APIServerTemplate, conf),
	}
}

func newAdminKubeConfigAsset(assets Assets, conf Config) (Asset, error) {
	caCert, err := assets.Get(AssetPathCACert)
	if err != nil {
		return Asset{}, err
	}

	adminCert, err := assets.Get(AssetPathAdminCert)
	if err != nil {
		return Asset{}, err
	}

	adminKey, err := assets.Get(AssetPathAdminKey)
	if err != nil {
		return Asset{}, err
	}

	templateCfg := struct {
		Server    string
		CACert    string
		UserName  string
		UserCert  string
		UserKey   string
		UserToken string
	}{
		Server:   conf.APIServers[0].String(),
		CACert:   base64.StdEncoding.EncodeToString(caCert.Data),
		UserName: "admin",
		UserCert: base64.StdEncoding.EncodeToString(adminCert.Data),
		UserKey:  base64.StdEncoding.EncodeToString(adminKey.Data),
	}

	return assetFromTemplate(AssetPathAdminKubeConfig, internal.KubeConfigTemplate, templateCfg)
}

func newBootstrapKubeConfigAsset(assets Assets, conf Config) (Asset, error) {
	caCert, err := assets.Get(AssetPathCACert)
	if err != nil {
		return Asset{}, err
	}

	tokenFile, err := assets.Get(AssetPathBootstrapAuthToken)
	if err != nil {
		return Asset{}, err
	}

	token, err := parseBootstrapAuthToken(tokenFile.Data)
	if err != nil {
		return Asset{}, err
	}

	templateCfg := struct {
		Server    string
		CACert    string
		UserName  string
		UserCert  string
		UserKey   string
		UserToken string
	}{
		Server:    conf.APIServers[0].String(),
		CACert:    base64.StdEncoding.EncodeToString(caCert.Data),
		UserName:  "kubelet-bootstrap",
		UserToken: token,
	}

	return assetFromTemplate(AssetPathBootstrapKubeConfig, internal.KubeConfigTemplate, templateCfg)
}

func newAPIServerSecretAsset(assets Assets) (Asset, error) {
	secretAssets := []string{
		AssetPathAPIServerKey,
		AssetPathAPIServerCert,
		AssetPathServiceAccountPubKey,
		AssetPathCACert,
		AssetPathBootstrapAuthToken,
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
		AssetPathCAKey,  // ^
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

func mustCreateAssetFromTemplate(name string, template []byte, data interface{}) Asset {
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
