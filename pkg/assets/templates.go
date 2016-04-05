package assets

import (
	"bytes"
	"encoding/base64"
	"path/filepath"
	"text/template"

	"github.com/ghodss/yaml"

	"github.com/coreos/bootkube/pkg/assets/internal"
)

var (
	secretNamespace     = "kube-system"
	secretAPIServerName = "kube-apiserver"
	secretCMName        = "kube-controllermanager"
)

func NewKubeConfig(assets Assets) (Asset, error) {
	var config Asset

	caCert, err := assets.Get(AssetPathCACert)
	if err != nil {
		return config, err
	}

	tmpl, err := template.New(AssetPathKubeConfig).Parse(string(internal.KubeConfigTemplate))
	if err != nil {
		return config, err
	}

	tmplConfig := struct {
		Server string
		Token  string
		CACert string
	}{
		//TODO(aaron): temporary hack. Get token info from generated asset & server from cli opt
		"https://172.17.4.100:6443",
		"token",
		base64.StdEncoding.EncodeToString(caCert.Data),
	}

	var data bytes.Buffer
	if err := tmpl.Execute(&data, tmplConfig); err != nil {
		return config, err
	}

	config.Name = AssetPathKubeConfig
	config.Data = data.Bytes()

	return config, nil
}

func NewAPIServerSecret(assets Assets) (Asset, error) {
	secretAssets := []string{
		AssetPathAPIServerKey,
		AssetPathAPIServerCert,
		AssetPathServiceAccountPubKey,
		AssetPathTokenAuth,
	}

	secretYAML, err := newSecretFromAssets(secretAPIServerName, secretNamespace, secretAssets, assets)
	if err != nil {
		return Asset{}, err
	}

	return Asset{Name: AssetPathAPIServerSecret, Data: secretYAML}, nil
}

func NewControllerManagerSecret(assets Assets) (Asset, error) {
	secretAssets := []string{
		AssetPathServiceAccountPrivKey,
		AssetPathCACert, //TODO(aaron): do we want this also distributed as secret? or expect available on host?
	}

	secretYAML, err := newSecretFromAssets(secretCMName, secretNamespace, secretAssets, assets)
	if err != nil {
		return Asset{}, err
	}

	return Asset{Name: AssetPathControllerManagerSecret, Data: secretYAML}, nil
}

func NewTokenAuth() Asset {
	// TODO(aaron): temp hack / should at minimum generate random token
	return Asset{
		Name: AssetPathTokenAuth,
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

func newSecretFromAssets(name, namespace string, assetNames []string, assets Assets) ([]byte, error) {
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
