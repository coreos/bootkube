package assets

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/coreos/bootkube/pkg/assets/internal"
)

const (
	AssetPathCAKey                   = "tls/ca.key"
	AssetPathCACert                  = "tls/ca.crt"
	AssetPathAPIServerKey            = "tls/apiserver.key"
	AssetPathAPIServerCert           = "tls/apiserver.crt"
	AssetPathServiceAccountPrivKey   = "tls/service-account.key"
	AssetPathServiceAccountPubKey    = "tls/service-account.pub"
	AssetPathKubeConfig              = "auth/kubeconfig.yaml"
	AssetPathTokenAuth               = "auth/token-auth.csv"
	AssetPathKubelet                 = "manifests/kubelet.yaml"
	AssetPathAPIServerSecret         = "manifests/kube-apiserver-secret.yaml"
	AssetPathAPIServer               = "manifests/kube-apiserver.yaml"
	AssetPathControllerManager       = "manifests/kube-controllermanager.yaml"
	AssetPathControllerManagerSecret = "manifests/kube-controllermanager-secret.yaml"
	AssetPathScheduler               = "manifests/kube-scheduler.yaml"
)

//go:generate go run templates_gen.go
//go:generate gofmt -w internal/templates.go

type Asset struct {
	Name string
	Data []byte
}

type Assets []Asset

func (a Assets) Get(name string) (Asset, error) {
	for _, asset := range a {
		if asset.Name == name {
			return asset, nil
		}
	}
	return Asset{}, fmt.Errorf("asset %q does not exist", name)
}

func (a Assets) WriteFiles(path string) error {
	if err := os.Mkdir(path, 0755); err != nil {
		return err
	}
	for _, asset := range a {
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

func StaticAssets() []Asset {
	return []Asset{
		{Name: AssetPathKubelet, Data: internal.KubeletTemplate},
		{Name: AssetPathAPIServer, Data: internal.APIServerTemplate},
		{Name: AssetPathControllerManager, Data: internal.ControllerManagerTemplate},
		{Name: AssetPathScheduler, Data: internal.SchedulerTemplate},
	}
}
