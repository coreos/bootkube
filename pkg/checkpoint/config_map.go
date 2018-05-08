package checkpoint

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const defaultConfigMapMode = os.FileMode(0600)

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
		if err := writeAndAtomicRename(filepath.Join(basePath, f), []byte(d), uid, gid, defaultConfigMapMode); err != nil {
			return "", fmt.Errorf("failed to write configMap %s: %v", configMap.Name, err)
		}
	}
	return basePath, nil
}

// checkpointConfigMapProjection will locally store configMap data from volumes with ProjectedVolumeSources.
// The path to the configmap data becomes: checkpointConfigMapPath/namespace/podname/volumename/configmap.file
// Where each "configmap.file" is a path from the ConfigMapProjection.KeyToPath field.
func (c *checkpointer) checkpointConfigMapProjection(namespace, podName, volumeName string, configMapProjection *v1.ConfigMapProjection, uid, gid int) error {

	configMapName := configMapProjection.Name
	isOptional := configMapProjection.Optional != nil && *configMapProjection.Optional

	configMap, err := c.apiserver.Core().ConfigMaps(namespace).Get(configMapName, metav1.GetOptions{})
	if err != nil && isOptional {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to retrieve configMap %s/%s: %v", namespace, configMapName, err)
	}

	basePath := configMapPath(namespace, podName, volumeName)
	if err = os.MkdirAll(basePath, 0700); err != nil {
		return fmt.Errorf("failed to create configMap checkpoint path %s: %v", basePath, err)
	}
	if err = os.Chown(basePath, uid, gid); err != nil {
		return fmt.Errorf("failed to chown configMap checkpoint path %s: %v", basePath, err)
	}

	// TODO(aaron): No need to store if already exists
	for _, item := range configMapProjection.Items {
		configMapItemKey := item.Key
		configMapItemMode := defaultConfigMapMode
		if item.Mode != nil {
			configMapItemMode = os.FileMode(*item.Mode)
		}
		if _, ok := configMap.Data[configMapItemKey]; !ok {
			if isOptional {
				continue
			}
			return fmt.Errorf("failed to find item %s configMap %s/%s: %v", configMapItemKey, namespace, configMapName, err)
		}
		if err := writeAndAtomicRename(filepath.Join(basePath, item.Path), []byte(configMap.Data[configMapItemKey]), uid, gid, configMapItemMode); err != nil {
			return fmt.Errorf("failed to write configMap %s: %v", configMap.Name, err)
		}
	}

	return nil
}

func (c *checkpointer) checkpointProjectedConfigMaps(namespace, podName, volumeName string, volumeSources []v1.VolumeProjection, uid, gid int) error {

	for _, vp := range volumeSources {
		err := c.checkpointConfigMapProjection(namespace, podName, volumeName, vp.ConfigMap, uid, gid)
		if err != nil {
			return fmt.Errorf("failed to checkpoint projected configMap for pod %s/%s: %v", namespace, podName, err)
		}
	}

	return nil
}

func configMapPath(namespace, podName, configMapName string) string {
	return filepath.Join(checkpointConfigMapPath, namespace, podName, configMapName)
}

func podFullNameToConfigMapPath(id string) string {
	namespace, podname := path.Split(id)
	return filepath.Join(checkpointConfigMapPath, namespace, podname)
}
