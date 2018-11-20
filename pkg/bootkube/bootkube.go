package bootkube

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/kubernetes-incubator/bootkube/pkg/asset"

	"k8s.io/client-go/tools/clientcmd"
)

const assetTimeout = 20 * time.Minute

type RequiredPodStatus int

const (
	PodStatusRunning RequiredPodStatus = iota
	PodStatusReady
)

type RequiredPod struct {
	Name      string
	Namespace string
	Status    RequiredPodStatus
}

type Config struct {
	AssetDir        string
	PodManifestPath string
	Strict          bool
	RequiredPods    []RequiredPod
}

type bootkube struct {
	podManifestPath string
	assetDir        string
	strict          bool
	requiredPods    []RequiredPod
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
			UserOutput("Error tearing down temporary bootstrap control plane: %v\n", err)
		}
	}()

	var err error
	defer func() {
		// Always report errors.
		if err != nil {
			UserOutput("Error: %v\n", err)
		}
	}()

	if err = bcp.Start(); err != nil {
		return err
	}

	if err = CreateAssets(kubeConfig, filepath.Join(b.assetDir, asset.AssetPathManifests), assetTimeout, b.strict); err != nil {
		return err
	}

	if err = WaitUntilPodsRunning(kubeConfig, b.requiredPods, assetTimeout); err != nil {
		return err
	}

	return nil
}

// All bootkube printing to stdout should go through this fmt.Printf wrapper.
// The stdout of bootkube should convey information useful to a human sitting
// at a terminal watching their cluster bootstrap itself. Otherwise the message
// should go to stderr.
func UserOutput(format string, a ...interface{}) {
	fmt.Printf(format, a...)
}
