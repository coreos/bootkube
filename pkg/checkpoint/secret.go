package checkpoint

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const defaultSecretMode = os.FileMode(0600)

// checkpointSecretVolumes ensures that all pod secrets are checkpointed locally, then converts the secret volume to a hostpath.
func (c *checkpointer) checkpointSecretVolumes(pod *v1.Pod) (*v1.Pod, error) {
	uid, gid, err := podUserAndGroup(pod)
	if err != nil {
		return nil, fmt.Errorf("failed to checkpoint secret for pod %s/%s: %v", pod.Namespace, pod.Name, err)
	}

	for i := range pod.Spec.Volumes {
		v := &pod.Spec.Volumes[i]
		if v.Secret == nil {
			continue
		}

		_, err := c.checkpointSecret(pod.Namespace, pod.Name, v.Secret.SecretName, uid, gid)
		if err != nil {
			return nil, fmt.Errorf("failed to checkpoint secret for pod %s/%s: %v", pod.Namespace, pod.Name, err)
		}

	}
	return pod, nil
}

// checkpointSecret will locally store secret data.
// The path to the secret data becomes: checkpointSecretPath/namespace/podname/secretName/secret.file
// Where each "secret.file" is a key from the secret.Data field.
func (c *checkpointer) checkpointSecret(namespace, podName, secretName string, uid, gid int) (string, error) {
	secret, err := c.apiserver.Core().Secrets(namespace).Get(secretName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to retrieve secret %s/%s: %v", namespace, secretName, err)
	}

	basePath := secretPath(namespace, podName, secretName)
	if err := os.MkdirAll(basePath, 0700); err != nil {
		return "", fmt.Errorf("failed to create secret checkpoint path %s: %v", basePath, err)
	}
	if err := os.Chown(basePath, uid, gid); err != nil {
		return "", fmt.Errorf("failed to chown secret checkpoint path %s: %v", basePath, err)
	}

	// TODO(aaron): No need to store if already exists
	for f, d := range secret.Data {
		if err := writeAndAtomicRename(filepath.Join(basePath, f), d, uid, gid, defaultSecretMode); err != nil {
			return "", fmt.Errorf("failed to write secret %s: %v", secret.Name, err)
		}
	}

	return basePath, nil
}

// checkpointSecretProjection will locally store secret data from volumes with ProjectedVolumeSources.
// The path to the secret data becomes: checkpointSecretPath/namespace/podname/volumename/secret.file
// Where each "secret.file" is a path from the SecretProjection.KeyToPath field.
func (c *checkpointer) checkpointSecretProjection(namespace, podName, volumeName string, secretProjection *v1.SecretProjection, uid, gid int) error {

	secretName := secretProjection.Name
	isOptional := secretProjection.Optional != nil && *secretProjection.Optional

	secret, err := c.apiserver.Core().Secrets(namespace).Get(secretName, metav1.GetOptions{})
	if err != nil && isOptional {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to retrieve secret %s/%s: %v", namespace, secretName, err)
	}

	basePath := secretPath(namespace, podName, volumeName)
	if err = os.MkdirAll(basePath, 0700); err != nil {
		return fmt.Errorf("failed to create secret checkpoint path %s: %v", basePath, err)
	}
	if err = os.Chown(basePath, uid, gid); err != nil {
		return fmt.Errorf("failed to chown secret checkpoint path %s: %v", basePath, err)
	}

	// TODO(aaron): No need to store if already exists
	for _, item := range secretProjection.Items {
		secretItemKey := item.Key
		secretItemMode := defaultSecretMode
		if item.Mode != nil {
			secretItemMode = os.FileMode(*item.Mode)
		}
		if _, ok := secret.Data[secretItemKey]; !ok {
			if isOptional {
				continue
			}
			return fmt.Errorf("failed to find item %s secret %s/%s: %v", secretItemKey, namespace, secretName, err)
		}
		if err := writeAndAtomicRename(filepath.Join(basePath, item.Path), secret.Data[secretItemKey], uid, gid, secretItemMode); err != nil {
			return fmt.Errorf("failed to write secret %s: %v", secret.Name, err)
		}
	}

	return nil
}

func (c *checkpointer) checkpointProjectedSecrets(namespace, podName, volumeName string, volumeSources []v1.VolumeProjection, uid, gid int) error {

	for _, vp := range volumeSources {
		err := c.checkpointSecretProjection(namespace, podName, volumeName, vp.Secret, uid, gid)
		if err != nil {
			return fmt.Errorf("failed to checkpoint projected secret for pod %s/%s: %v", namespace, podName, err)
		}
	}

	return nil
}

func secretPath(namespace, podName, secretName string) string {
	return filepath.Join(checkpointSecretPath, namespace, podName, secretName)
}

func podFullNameToSecretPath(id string) string {
	namespace, podname := path.Split(id)
	return filepath.Join(checkpointSecretPath, namespace, podname)
}
