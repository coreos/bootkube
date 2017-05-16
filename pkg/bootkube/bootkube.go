package bootkube

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"k8s.io/client-go/tools/clientcmd"

	"github.com/kubernetes-incubator/bootkube/pkg/asset"
	"github.com/kubernetes-incubator/bootkube/pkg/util/etcdutil"
)

const assetTimeout = 20 * time.Minute

var kubeConfig clientcmd.ClientConfig

var requiredPods = []string{
	"pod-checkpointer",
	"kube-apiserver",
	"kube-scheduler",
	"kube-controller-manager",
}

type Config struct {
	AssetDir        string
	PodManifestPath string
}

type bootkube struct {
	podManifestPath string
	assetDir        string
}

func NewBootkube(config Config) (*bootkube, error) {
	return &bootkube{
		assetDir:        config.AssetDir,
		podManifestPath: config.PodManifestPath,
	}, nil
}

func (b *bootkube) Run() error {
	// TODO(diegs): create and share a single client rather than the kubeconfig once all uses of it
	// are migrated to client-go.
	kubeConfig = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: filepath.Join(b.assetDir, asset.AssetPathKubeConfig)},
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

	if err = CreateAssets(filepath.Join(b.assetDir, asset.AssetPathManifests), assetTimeout); err != nil {
		return err
	}

	selfHostedEtcd, err := detectSelfHostedEtcd(b.assetDir, asset.AssetPathBootstrapEtcd)
	if err != nil {
		return err
	}

	if selfHostedEtcd {
		requiredPods = append(requiredPods, "etcd-operator")
	}

	if err = WaitUntilPodsRunning(requiredPods, assetTimeout); err != nil {
		return err
	}

	if selfHostedEtcd {
		UserOutput("Migrating to self-hosted etcd cluster...\n")
		if err = etcdutil.Migrate(kubeConfig, filepath.Join(b.assetDir, asset.AssetPathBootstrapEtcdService), filepath.Join(b.assetDir, asset.AssetPathMigrateEtcdCluster)); err != nil {
			return err
		}
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

// detectSelfHostedEtcd returns true if the asset dir contains assets for bootstrap etcd.
func detectSelfHostedEtcd(assetDir, assetPathBootstrapEtcd string) (bool, error) {
	etcdAssetsPath := filepath.Join(assetDir, assetPathBootstrapEtcd)
	_, err := os.Stat(etcdAssetsPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
