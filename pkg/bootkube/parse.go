package bootkube

import (
	"fmt"
	"io/ioutil"
	"net"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/kubernetes-incubator/bootkube/pkg/asset"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
)

const (
	apiServerContainerName         = "kube-apiserver"
	controllerManagerContainerName = "kube-controller-manager"
)

// detectServiceCIDR deserializes the '--service-cluster-ip-range' from the
// kube-apiserver manifest
func detectServiceCIDR(config Config) (string, error) {
	b, err := ioutil.ReadFile(filepath.Join(config.AssetDir, asset.AssetPathAPIServer))
	if err != nil {
		return "", fmt.Errorf("can't read file %s: %v", asset.AssetPathAPIServer, err)
	}
	var apiServer v1beta1.DaemonSet
	err = yaml.Unmarshal(b, &apiServer)
	if err != nil {
		return "", fmt.Errorf("cant unmarshal %s: %v", asset.AssetPathAPIServer, err)
	}

	containers := map[string]v1.Container{}
	for _, container := range apiServer.Spec.Template.Spec.Containers {
		containers[container.Name] = container
	}

	if container, exists := containers[apiServerContainerName]; exists {
		cidr := findFlag("--service-cluster-ip-range", container.Command)
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return "", fmt.Errorf("invalid --cluster-cidr CIDR: %v", err)
		}
		return cidr, nil
	}
	return "", fmt.Errorf("can't detect --service-cluster-ip-range in %s", asset.AssetPathAPIServer)
}

// detectPodCIDR deserializes the '--cluster-cidr' from the
// kube-controller-manager manifest.
func detectPodCIDR(config Config) (string, error) {
	b, err := ioutil.ReadFile(filepath.Join(config.AssetDir, asset.AssetPathControllerManager))
	if err != nil {
		return "", fmt.Errorf("can't read file %s: %v", asset.AssetPathControllerManager, err)
	}
	var manager v1beta1.Deployment
	err = yaml.Unmarshal(b, &manager)
	if err != nil {
		return "", fmt.Errorf("can't unmarshal %s: %v", asset.AssetPathControllerManager, err)
	}

	containers := map[string]v1.Container{}
	for _, container := range manager.Spec.Template.Spec.Containers {
		containers[container.Name] = container
	}

	if container, exists := containers[controllerManagerContainerName]; exists {
		cidr := findFlag("--cluster-cidr", container.Command)
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return "", fmt.Errorf("invalid --cluster-cidr CIDR: %v", err)
		}
		return cidr, nil
	}
	return "", fmt.Errorf("can't detect --cluster-cidr flag in %s", asset.AssetPathControllerManager)
}

// detectEtcdIP deserializes the etcd-service ClusterIP.
func detectEtcdIP(assetDir string) (string, error) {
	b, err := ioutil.ReadFile(filepath.Join(assetDir, asset.AssetPathEtcdSvc))
	if err != nil {
		return "", fmt.Errorf("can't read file %s: %v", asset.AssetPathEtcdSvc, err)
	}
	var service v1.Service
	err = yaml.Unmarshal(b, &service)
	if err != nil {
		return "", fmt.Errorf("can't unmarshal %s: %v", asset.AssetPathEtcdSvc, err)
	}
	return service.Spec.ClusterIP, nil
}

func findFlag(flagName string, args []string) string {
	for _, arg := range args {
		if strings.HasPrefix(arg, flagName+"=") {
			return strings.TrimPrefix(arg, flagName+"=")
		}
		if strings.HasPrefix(arg, flagName+" ") {
			return strings.TrimSpace(strings.TrimPrefix(arg, flagName+" "))
		}
	}
	return ""
}
