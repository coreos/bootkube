package bootkube

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kubernetes-incubator/bootkube/pkg/asset"
)

var (
	secrets   = []string{"secret-1.yaml", "secret-2.yaml", "secret-3.yaml"}
	manifests = []string{"pod-1.yaml", "pod-2.yaml"}
)

func setUp(t *testing.T) (assetDir, podManifestPath string) {
	// Create source directories.
	var err error
	assetDir, err = ioutil.TempDir("", "assets")
	if err != nil {
		t.Fatal(err)
	}
	podManifestPath, err = ioutil.TempDir("", "manifests")
	if err != nil {
		t.Fatal(err)
	}
	asset.BootstrapSecretsDir, err = ioutil.TempDir("", "bootstrap-secrets")
	if err != nil {
		t.Fatal(err)
	}

	// Create assets.
	if err := os.Mkdir(filepath.Join(assetDir, asset.AssetPathSecrets), os.FileMode(0755)); err != nil {
		t.Fatal(err)
	}
	for _, secret := range secrets {
		if err := ioutil.WriteFile(filepath.Join(assetDir, asset.AssetPathSecrets, secret), []byte("secret data"), os.FileMode(0644)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(assetDir, asset.AssetPathBootstrapManifests), os.FileMode(0755)); err != nil {
		t.Fatal(err)
	}
	for _, manifest := range manifests {
		if err := ioutil.WriteFile(filepath.Join(assetDir, asset.AssetPathBootstrapManifests, manifest), []byte("manifest data"), os.FileMode(0644)); err != nil {
			t.Fatal(err)
		}
	}
	return
}

func tearDown(assetDir, podManifestPath string, t *testing.T) {
	if err := os.RemoveAll(assetDir); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(podManifestPath); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(asset.BootstrapSecretsDir); err != nil {
		t.Fatal(err)
	}
}

func TestBootstrapControlPlane(t *testing.T) {
	assetDir, podManifestPath := setUp(t)
	defer tearDown(assetDir, podManifestPath, t)

	// Create and start bootstrap control plane.
	bcp := NewBootstrapControlPlane(assetDir, podManifestPath)
	if err := bcp.Start(); err != nil {
		t.Errorf("bcp.Start() = %v, want: nil", err)
	}

	// Make sure assets were copied.
	for _, secret := range secrets {
		if _, err := os.Stat(filepath.Join(asset.BootstrapSecretsDir, secret)); os.IsNotExist(err) {
			t.Errorf("bcp.Start() failed to copy secret: %v", secret)
		}
	}
	for _, manifest := range manifests {
		if _, err := os.Stat(filepath.Join(podManifestPath, manifest)); os.IsNotExist(err) {
			t.Errorf("bcp.Start() failed to copy manifest: %v", manifest)
		}
	}

	// Tear down control plane.
	if err := bcp.Teardown(); err != nil {
		t.Errorf("bcp.Teardown() = %v, want: nil", err)
	}

	// Make sure directories were properly cleaned up.
	if fi, err := os.Stat(asset.BootstrapSecretsDir); fi != nil || !os.IsNotExist(err) {
		t.Error("bcp.Teardown() failed to delete secrets directory")
	}
	for _, manifest := range manifests {
		if fi, err := os.Stat(filepath.Join(podManifestPath, manifest)); fi != nil || !os.IsNotExist(err) {
			t.Errorf("bcp.Teardown() failed to delete manifest: %v", manifest)
		}
	}
}

func TestBootstrapControlPlaneNoOverwrite(t *testing.T) {
	assetDir, podManifestPath := setUp(t)
	defer tearDown(assetDir, podManifestPath, t)
	existingManifest := manifests[1]
	existingData := []byte("existing data")

	// Create a manifest in the destination already.
	if err := ioutil.WriteFile(filepath.Join(podManifestPath, existingManifest), existingData, os.FileMode(0644)); err != nil {
		t.Fatal(err)
	}

	// Create and start bootstrap control plane.
	bcp := NewBootstrapControlPlane(assetDir, podManifestPath)
	if err := bcp.Start(); err == nil {
		t.Errorf("bcp.Start() = %v, want: non-nil", err)
	}

	// Make sure assets were copied.
	for _, secret := range secrets {
		if _, err := os.Stat(filepath.Join(asset.BootstrapSecretsDir, secret)); os.IsNotExist(err) {
			t.Errorf("bcp.Start() failed to copy secret: %v", secret)
		}
	}
	for _, manifest := range manifests {
		if _, err := os.Stat(filepath.Join(podManifestPath, manifest)); os.IsNotExist(err) {
			t.Errorf("bcp.Start() failed to copy manifest: %v", manifest)
		}
		if manifest == existingManifest {
			data, err := ioutil.ReadFile(filepath.Join(podManifestPath, manifest))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(data, existingData) {
				t.Errorf("existing manifest %v was overwritten, got: %s, want: %s", existingManifest, data, existingData)
			}
		}
	}

	// Tear down control plane.
	if err := bcp.Teardown(); err != nil {
		t.Errorf("bcp.Start() = %v, want: nil", err)
	}

	// Make sure directories were properly cleaned up.
	if fi, err := os.Stat(asset.BootstrapSecretsDir); fi != nil || !os.IsNotExist(err) {
		t.Error("bcp.Teardown() failed to delete secrets directory")
	}
	for _, manifest := range manifests {
		if manifest == existingManifest {
			continue // The manifest previously existed -- do not delete.
		}
		if fi, err := os.Stat(filepath.Join(podManifestPath, manifest)); fi != nil || !os.IsNotExist(err) {
			t.Errorf("bcp.Teardown() failed to delete manifest: %v", manifest)
		}
	}
}
