package bootkube

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/ghodss/yaml"
	"github.com/kubernetes-incubator/bootkube/pkg/asset"
	"k8s.io/client-go/pkg/api/v1"
)

// detectEtcdIP deserializes the etcd-service ClusterIP.
func detectEtcdIP(assetDir string) (string, error) {
	path := filepath.Join(assetDir, asset.AssetPathEtcdSvc)
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("can't read file %s: %v", path, err)
	}
	var service v1.Service
	err = yaml.Unmarshal(b, &service)
	if err != nil {
		return "", fmt.Errorf("can't unmarshal %s: %v", path, err)
	}
	return service.Spec.ClusterIP, nil
}
