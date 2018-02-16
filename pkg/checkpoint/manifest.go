package checkpoint

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
)

// getFileCheckpoints will retrieve all checkpoint manifests from a given filepath.
func getFileCheckpoints(path string) map[string]*v1.Pod {
	checkpoints := make(map[string]*v1.Pod)

	fi, err := ioutil.ReadDir(path)
	if err != nil {
		glog.Fatalf("Failed to read checkpoint manifest path: %v", err)
	}

	for _, f := range fi {
		manifest := filepath.Join(path, f.Name())

		// Check for leftover temporary checkpoints.
		if strings.HasPrefix(filepath.Base(manifest), ".") {
			glog.V(4).Infof("Found temporary checkpoint %s, removing.", manifest)
			if err := os.Remove(manifest); err != nil {
				glog.V(4).Infof("Error removing temporary checkpoint %s: %v.", manifest, err)
			}
			continue
		}

		b, err := ioutil.ReadFile(manifest)
		if err != nil {
			glog.Errorf("Error reading manifest: %v", err)
			continue
		}

		cp := &v1.Pod{}
		if err := runtime.DecodeInto(legacyscheme.Codecs.UniversalDecoder(), b, cp); err != nil {
			glog.Errorf("Error unmarshalling manifest from %s: %v", filepath.Join(path, f.Name()), err)
			continue
		}

		if isCheckpoint(cp) {
			if _, ok := checkpoints[podFullName(cp)]; ok { // sanity check
				glog.Warningf("Found multiple checkpoint pods in %s with same id: %s", path, podFullName(cp))
			}
			checkpoints[podFullName(cp)] = cp
		}
	}
	return checkpoints
}

// writeCheckpointManifest will save the pod to the inactive checkpoint location if it doesn't already exist.
func writeCheckpointManifest(pod *v1.Pod) (bool, error) {
	buff := &bytes.Buffer{}
	if err := podSerializer.Encode(pod, buff); err != nil {
		return false, err
	}
	path := filepath.Join(inactiveCheckpointPath, pod.Namespace+"-"+pod.Name+".json")
	return writeManifestIfDifferent(path, podFullName(pod), buff.Bytes())
}

// writeManifestIfDifferent writes `data` to `path` if data is different from the existing content.
// The `name` parameter is used for debug output.
func writeManifestIfDifferent(path, name string, data []byte) (bool, error) {
	existing, err := ioutil.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if bytes.Equal(existing, data) {
		glog.V(4).Infof("Checkpoint manifest for %q already exists. Skipping", name)
		return false, nil
	}
	glog.Infof("Writing manifest for %q to %q", name, path)
	return true, writeAndAtomicRename(path, data, rootUID, rootGID, 0644)
}

func writeAndAtomicRename(path string, data []byte, uid, gid int, perm os.FileMode) error {
	// Ensure that the temporary file is on the same filesystem so that os.Rename() does not error.
	tmpfile, err := ioutil.TempFile(filepath.Dir(path), ".")
	if err != nil {
		return err
	}
	if _, err := tmpfile.Write(data); err != nil {
		return err
	}
	if err := tmpfile.Chmod(perm); err != nil {
		return err
	}
	if err := tmpfile.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpfile.Name(), path); err != nil {
		return err
	}
	return os.Chown(path, uid, gid)
}
