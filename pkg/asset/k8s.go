package asset

import (
	"bytes"
	"encoding/base64"
	"path/filepath"
	"text/template"

	"github.com/ghodss/yaml"

	"github.com/coreos/bootkube/pkg/asset/internal"
)

const (
	secretNamespace     = "kube-system"
	secretAPIServerName = "kube-apiserver"
	secretCMName        = "kube-controllermanager"
)

func newStaticAssets() Assets {
	var noData interface{}
	return Assets{
		mustCreateAssetFromTemplate(assetPathControllerManager, internal.ControllerManagerTemplate, noData),
		mustCreateAssetFromTemplate(assetPathScheduler, internal.SchedulerTemplate, noData),
		mustCreateAssetFromTemplate(assetPathKubeDNSRc, internal.DNSRcTemplate, noData),
		mustCreateAssetFromTemplate(assetPathKubeDNSSvc, internal.DNSSvcTemplate, noData),
		mustCreateAssetFromTemplate(assetPathSystemNamespace, internal.SystemNSTemplate, noData),
	}
}

func newDynamicAssets(conf Config) Assets {
	return Assets{
		mustCreateAssetFromTemplate(assetPathKubelet, internal.KubeletTemplate, conf),
		mustCreateAssetFromTemplate(assetPathAPIServer, internal.APIServerTemplate, conf),
		mustCreateAssetFromTemplate(assetPathProxy, internal.ProxyTemplate, conf),
	}
}

func newKubeConfigAsset(assets Assets) (Asset, error) {
	caCert, err := assets.Get(assetPathCACert)
	if err != nil {
		return Asset{}, err
	}

	data := struct {
		Server string
		Token  string
		CACert string
	}{
		//TODO(aaron): temporary hack. Get token info from generated asset & server from cli opt
		"https://172.17.4.100:6443",
		"token",
		base64.StdEncoding.EncodeToString(caCert.Data),
	}

	return assetFromTemplate(assetPathKubeConfig, internal.KubeConfigTemplate, data)
}

func newAPIServerSecretAsset(assets Assets) (Asset, error) {
	secretAssets := []string{
		assetPathAPIServerKey,
		assetPathAPIServerCert,
		assetPathServiceAccountPubKey,
		assetPathTokenAuth,
	}

	secretYAML, err := secretFromAssets(secretAPIServerName, secretNamespace, secretAssets, assets)
	if err != nil {
		return Asset{}, err
	}

	return Asset{Name: assetPathAPIServerSecret, Data: secretYAML}, nil
}

func newControllerManagerSecretAsset(assets Assets) (Asset, error) {
	secretAssets := []string{
		assetPathServiceAccountPrivKey,
		assetPathCACert, //TODO(aaron): do we want this also distributed as secret? or expect available on host?
	}

	secretYAML, err := secretFromAssets(secretCMName, secretNamespace, secretAssets, assets)
	if err != nil {
		return Asset{}, err
	}

	return Asset{Name: assetPathControllerManagerSecret, Data: secretYAML}, nil
}

func newTokenAuthAsset() Asset {
	// TODO(aaron): temp hack / should at minimum generate random token
	return Asset{
		Name: assetPathTokenAuth,
		Data: []byte("token,admin,1"),
	}
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

func mustCreateAssetFromTemplate(name string, template []byte, data interface{}) Asset {
	a, err := assetFromTemplate(name, template, data)
	if err != nil {
		panic(err)
	}
	return a
}
