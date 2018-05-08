package checkpoint

import (
	"fmt"

	"k8s.io/api/core/v1"
)

func (c *checkpointer) checkpointProjectedVolumes(pod *v1.Pod) (*v1.Pod, error) {
	uid, gid, err := podUserAndGroup(pod)
	if err != nil {
		return nil, fmt.Errorf("failed to get user and group for pod %s/%s: %v", pod.Namespace, pod.Name, err)
	}

	for i := range pod.Spec.Volumes {
		v := &pod.Spec.Volumes[i]

		if v.VolumeSource.Projected != nil {
			p := v.VolumeSource.Projected
			if volumeHasSecretProjection(v) {
				err := c.checkpointProjectedSecrets(pod.Namespace, pod.Name, v.Name, p.Sources, uid, gid)
				if err != nil {
					return nil, fmt.Errorf("failed to checkpoint projected secrets for pod %s/%s: %v", pod.Namespace, pod.Name, err)
				}
			}
			if volumeHasConfigMapProjection(v) {
				err := c.checkpointProjectedConfigMaps(pod.Namespace, pod.Name, v.Name, p.Sources, uid, gid)
				if err != nil {
					return nil, fmt.Errorf("failed to checkpoint projected configmaps for pod %s/%s: %v", pod.Namespace, pod.Name, err)
				}
			}
			if volumeHasDownwardAPIProjection(v) {
				return nil, fmt.Errorf("no support to checkpoint projected DownwardAPI for pod %s/%s", pod.Namespace, pod.Name)
			}
		}
	}
	return pod, nil
}

func volumeHasSecretProjection(volume *v1.Volume) bool {
	hasSecretProjection := false
	if volume.Projected != nil {
		for _, s := range volume.Projected.Sources {
			if s.Secret != nil {
				hasSecretProjection = true
				break
			}
		}
	}
	return hasSecretProjection
}

func volumeHasConfigMapProjection(volume *v1.Volume) bool {
	hasConfigMapProjection := false
	if volume.Projected != nil {
		for _, s := range volume.Projected.Sources {
			if s.ConfigMap != nil {
				hasConfigMapProjection = true
				break
			}
		}
	}
	return hasConfigMapProjection
}

func volumeHasDownwardAPIProjection(volume *v1.Volume) bool {
	hasDownwardAPIProjection := false
	if volume.Projected != nil {
		for _, s := range volume.Projected.Sources {
			if s.DownwardAPI != nil {
				hasDownwardAPIProjection = true
				break
			}
		}
	}
	return hasDownwardAPIProjection
}
