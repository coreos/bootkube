package checkpoint

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// checkpointConfigMapVolumes ensures that all pod configMaps are checkpointed locally, then converts the configMap volume to a hostpath.
func (c *checkpointer) checkpointConfigMapVolumes(pod *v1.Pod) (*v1.Pod, error) {
	uid, gid, err := podUserAndGroup(pod)
	if err != nil {
		return nil, fmt.Errorf("failed to checkpoint configMap for pod %s/%s: %v", pod.Namespace, pod.Name, err)
	}

	for i := range pod.Spec.Volumes {
		v := &pod.Spec.Volumes[i]
		if v.ConfigMap == nil {
			continue
		}

		_, err := c.checkpointConfigMap(pod.Namespace, pod.Name, v.ConfigMap.Name, uid, gid)
		if err != nil {
			return nil, fmt.Errorf("failed to checkpoint configMap for pod %s/%s: %v", pod.Namespace, pod.Name, err)
		}
	}
	return pod, nil
}

// checkpointConfigMap will locally store configMap data.
// The path to the configMap data becomes: checkpointConfigMapPath/namespace/podname/configMapName/configMap.file
// Where each "configMap.file" is a key from the configMap.Data field.
func (c *checkpointer) checkpointConfigMap(namespace, podName, configMapName string, uid, gid int) (string, error) {
	configMap, err := c.apiserver.Core().ConfigMaps(namespace).Get(configMapName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to retrieve configMap %s/%s: %v", namespace, configMapName, err)
	}

	basePath := configMapPath(namespace, podName, configMapName)
	if err := os.MkdirAll(basePath, 0700); err != nil {
		return "", fmt.Errorf("failed to create configMap checkpoint path %s: %v", basePath, err)
	}
	if err := os.Chown(basePath, uid, gid); err != nil {
		return "", fmt.Errorf("failed to chown configMap checkpoint path %s: %v", basePath, err)
	}

	// TODO(aaron): No need to store if already exists
	for f, d := range configMap.Data {
		if err := writeAndAtomicRename(filepath.Join(basePath, f), []byte(d), uid, gid, 0600); err != nil {
			return "", fmt.Errorf("failed to write configMap %s: %v", configMap.Name, err)
		}
	}
	return basePath, nil
}

func configMapPath(namespace, podName, configMapName string) string {
	return filepath.Join(checkpointConfigMapPath, namespace, podName, configMapName)
}

func podFullNameToConfigMapPath(id string) string {
	namespace, podname := path.Split(id)
	return filepath.Join(checkpointConfigMapPath, namespace, podname)
}
