package bootkube

import (
	"path/filepath"
	"time"

	"github.com/kubernetes-sigs/bootkube/cmd/render/plugin/default/asset"
	"github.com/kubernetes-sigs/bootkube/pkg/helm"
	"github.com/kubernetes-sigs/bootkube/pkg/util"

	"k8s.io/client-go/tools/clientcmd"
)

const assetTimeout = 20 * time.Minute

type Config struct {
	AssetDir        string
	PodManifestPath string
	Strict          bool
	RequiredPods    []string
}

type bootkube struct {
	podManifestPath string
	assetDir        string
	strict          bool
	requiredPods    []string
}

func NewBootkube(config Config) (*bootkube, error) {
	return &bootkube{
		assetDir:        config.AssetDir,
		podManifestPath: config.PodManifestPath,
		strict:          config.Strict,
		requiredPods:    config.RequiredPods,
	}, nil
}

func (b *bootkube) Run() error {
	// TODO(diegs): create and share a single client rather than the kubeconfig once all uses of it
	// are migrated to client-go.
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: filepath.Join(b.assetDir, asset.AssetPathAdminKubeConfig)},
		&clientcmd.ConfigOverrides{})

	bcp := NewBootstrapControlPlane(b.assetDir, b.podManifestPath)

	defer func() {
		// Always tear down the bootstrap control plane and clean up manifests and secrets.
		if err := bcp.Teardown(); err != nil {
			util.UserOutput("Error tearing down temporary bootstrap control plane: %v\n", err)
		}
	}()

	var err error
	defer func() {
		// Always report errors.
		if err != nil {
			util.UserOutput("Error: %v\n", err)
		}
	}()

	if err = bcp.Start(); err != nil {
		return err
	}
	// wait for the api server to be up
	if err := util.PollApiServerUntilTimeout(kubeConfig, assetTimeout); err != nil {
		return err
	}

	if err = CreateAssets(kubeConfig, filepath.Join(b.assetDir, asset.AssetPathManifests), b.strict); err != nil {
		return err
	}

	kubeconfigPath := filepath.Join(b.assetDir, asset.AssetPathAdminKubeConfig)
	if err = helm.InstallCharts(kubeconfigPath, kubeConfig, filepath.Join(b.assetDir, asset.AssetPathCharts)); err != nil {
		return err
	}

	if err = WaitUntilPodsRunning(kubeConfig, b.requiredPods, assetTimeout); err != nil {
		return err
	}

	return nil
}
