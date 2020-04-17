package bootkube

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/kubernetes-sigs/bootkube/cmd/render/plugin/default/asset"
	"github.com/kubernetes-sigs/bootkube/pkg/util"
)

type bootstrapControlPlane struct {
	assetDir        string
	podManifestPath string
	ownedManifests  []string
}

// NewBootstrapControlPlane constructs a new bootstrap control plane object.
func NewBootstrapControlPlane(assetDir, podManifestPath string) *bootstrapControlPlane {
	return &bootstrapControlPlane{
		assetDir:        assetDir,
		podManifestPath: podManifestPath,
	}
}

// Start seeds static manifests to the kubelet to launch the bootstrap control plane.
// Users should always ensure that Cleanup() is called even in the case of errors.
func (b *bootstrapControlPlane) Start() error {
	util.UserOutput("Starting temporary bootstrap control plane...\n")
	// Make secrets temporarily available to bootstrap cluster.
	if err := os.RemoveAll(asset.BootstrapSecretsDir); err != nil {
		return err
	}
	secretsDir := filepath.Join(b.assetDir, asset.AssetPathSecrets)
	if _, err := copyDirectory(secretsDir, asset.BootstrapSecretsDir, true /* overwrite */); err != nil {
		return err
	}
	// Copy the admin kubeconfig. TODO(diegs): this is kind of a hack, maybe do something better.
	if err := copyFile(filepath.Join(b.assetDir, asset.AssetPathAdminKubeConfig), filepath.Join(asset.BootstrapSecretsDir, "kubeconfig"), true /* overwrite */); err != nil {
		return err
	}

	// Copy the static manifests to the kubelet's pod manifest path.
	manifestsDir := filepath.Join(b.assetDir, asset.AssetPathBootstrapManifests)
	ownedManifests, err := copyDirectory(manifestsDir, b.podManifestPath, false /* overwrite */)
	b.ownedManifests = ownedManifests // always copy in case of partial failure.
	return err
}

// Teardown brings down the bootstrap control plane and cleans up the temporary manifests and
// secrets. This function is idempotent.
func (b *bootstrapControlPlane) Teardown() error {
	util.UserOutput("Tearing down temporary bootstrap control plane...\n")
	if err := os.RemoveAll(asset.BootstrapSecretsDir); err != nil {
		return err
	}
	for _, manifest := range b.ownedManifests {
		if err := os.Remove(manifest); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	b.ownedManifests = nil
	return nil
}

// copyFile copies a single file from src to dst. Returns an error if overwrite is true and dst
// exists, or if any I/O error occurs during copying.
func copyFile(src, dst string, overwrite bool) error {
	flags := os.O_CREATE | os.O_WRONLY
	if !overwrite {
		flags |= os.O_EXCL
	}

	dstfile, err := os.OpenFile(dst, flags, os.FileMode(0600))
	if err != nil {
		return err
	}
	defer dstfile.Close()

	srcfile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcfile.Close()

	_, err = io.Copy(dstfile, srcfile)
	return err
}

// copyDirectory copies srcDir to dstDir recursively. It returns the paths of files (not
// directories) that were copied.
func copyDirectory(srcDir, dstDir string, overwrite bool) ([]string, error) {
	var copied []string
	return copied, filepath.Walk(srcDir, func(src string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		dst := filepath.Join(dstDir, strings.TrimPrefix(src, srcDir))
		if info.IsDir() {
			err = os.Mkdir(dst, os.FileMode(0700))
			if os.IsExist(err) {
				err = nil
			}
			return err
		}
		if err := copyFile(src, dst, overwrite); err != nil {
			return err
		}
		copied = append(copied, dst)
		return nil
	})
}
