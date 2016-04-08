package asset

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

const (
	assetPathCAKey                   = "tls/ca.key"
	assetPathCACert                  = "tls/ca.crt"
	assetPathAPIServerKey            = "tls/apiserver.key"
	assetPathAPIServerCert           = "tls/apiserver.crt"
	assetPathServiceAccountPrivKey   = "tls/service-account.key"
	assetPathServiceAccountPubKey    = "tls/service-account.pub"
	assetPathKubeConfig              = "auth/kubeconfig.yaml"
	assetPathTokenAuth               = "auth/token-auth.csv"
	assetPathKubelet                 = "manifests/kubelet.yaml"
	assetPathProxy                   = "manifests/kube-proxy.yaml"
	assetPathAPIServerSecret         = "manifests/kube-apiserver-secret.yaml"
	assetPathAPIServer               = "manifests/kube-apiserver.yaml"
	assetPathControllerManager       = "manifests/kube-controllermanager.yaml"
	assetPathControllerManagerSecret = "manifests/kube-controllermanager-secret.yaml"
	assetPathScheduler               = "manifests/kube-scheduler.yaml"
	assetPathKubeDNSRc               = "manifests/kube-dns-rc.yaml"
	assetPathKubeDNSSvc              = "manifests/kube-dns-svc.yaml"
	assetPathSystemNamespace         = "manifests/kube-system-ns.yaml"
)

// AssetConfig holds all configuration needed when generating
// the default set of assets.
type Config struct {
	APIServerCertIPAddrs string
	ETCDServers          string
	APIServers           string
}

// NewDefaultAssets returns a list of default assets, optionally
// configured via a user provided AssetConfig. Default assets include
// TLS assets (certs, keys and secrets), token authentication assets,
// and k8s component manifests.
func NewDefaultAssets(conf Config) (Assets, error) {
	// Static Assets
	as := newStaticAssets()
	as = append(as, newDynamicAssets(conf)...)

	// TLS assets
	tlsAssets, err := newTLSAssets(strings.Split(conf.APIServerCertIPAddrs, ","))
	if err != nil {
		return Assets{}, err
	}
	as = append(as, tlsAssets...)

	// Token Auth
	as = append(as, newTokenAuthAsset())

	// K8S kubeconfig
	kubeConfig, err := newKubeConfigAsset(as)
	if err != nil {
		return Assets{}, err
	}
	as = append(as, kubeConfig)

	// K8S APIServer secret
	apiSecret, err := newAPIServerSecretAsset(as)
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
		f := filepath.Join(path, asset.Name)
		if err := os.MkdirAll(filepath.Dir(f), 0755); err != nil {
			return err
		}
		fmt.Printf("Writing asset: %s\n", f)
		if err := ioutil.WriteFile(f, asset.Data, 0600); err != nil {
			return err
		}
	}
	return nil
}
