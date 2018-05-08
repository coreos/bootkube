package checkpoint

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"
)

var (
	// podSerializer is an encoder for writing checkpointed pods.
	//
	// Perfer this instead of json.Marshal because it corrects metadata before
	// serializing. For example it automatically fills in the "apiVersion" field.
	podSerializer = scheme.Codecs.EncoderForVersion(
		json.NewSerializer(
			json.DefaultMetaFactory,
			scheme.Scheme, // client-go's default scheme.
			scheme.Scheme,
			false, // don't pretty print.
		),
		v1.SchemeGroupVersion,
	)
)

func sanitizeCheckpointPod(cp *v1.Pod) *v1.Pod {
	trueVar := true

	// Check if this is already sanitized, i.e. it was read back from a checkpoint on disk.
	if _, ok := cp.Annotations[checkpointParentAnnotation]; ok {
		return cp
	}

	// Keep same name, namespace, and labels as parent.
	cp.ObjectMeta = metav1.ObjectMeta{
		Name:        cp.Name,
		Namespace:   cp.Namespace,
		Annotations: make(map[string]string),
		Labels:      cp.Labels,
		// Set the ownerRef to the parent pod. We do this because:
		// If the ownerRef stays the same (e.g. the original deployment), then the deployment controller will try to manage the static/mirror pod.
		// If we clear the ownerRef, then a higher-level object will adopt this pod based on the label selector (e.g. the original deployment).
		OwnerReferences: []metav1.OwnerReference{
			{
				APIVersion: cp.APIVersion,
				Kind:       cp.Kind,
				Name:       cp.Name,
				UID:        cp.UID,
				Controller: &trueVar,
			},
		},
	}

	// Track this checkpoint's parent pod
	cp.Annotations[checkpointParentAnnotation] = cp.Name

	// Remove Service Account
	cp.Spec.ServiceAccountName = ""
	cp.Spec.DeprecatedServiceAccount = ""

	// Sanitize the volumes
	for i := range cp.Spec.Volumes {
		v := &cp.Spec.Volumes[i]
		if v.Secret != nil {
			v.HostPath = &v1.HostPathVolumeSource{Path: secretPath(cp.Namespace, cp.Name, v.Secret.SecretName)}
			v.Secret = nil
		} else if v.ConfigMap != nil {
			v.HostPath = &v1.HostPathVolumeSource{Path: configMapPath(cp.Namespace, cp.Name, v.ConfigMap.Name)}
			v.ConfigMap = nil
		} else if v.Projected != nil {
			// can only currently support projected volumes with all secrets or all configmaps
			if volumeHasSecretProjection(v) {
				v.HostPath = &v1.HostPathVolumeSource{Path: secretPath(cp.Namespace, cp.Name, v.Name)}
				v.Projected = nil
			} else if volumeHasConfigMapProjection(v) {
				v.HostPath = &v1.HostPathVolumeSource{Path: configMapPath(cp.Namespace, cp.Name, v.Name)}
				v.Projected = nil
			}
		}
	}

	// Clear pod status
	cp.Status.Reset()

	return cp
}

// isPodCheckpointer returns true if the manifest is the pod checkpointer (has the same name as the parent).
// For example, the pod.Name would be "pod-checkpointer".
// The podName would be "pod-checkpointer" or "pod-checkpointer-172.17.4.201" where
// "172.17.4.201" is the nodeName.
func isPodCheckpointer(pod *v1.Pod, cp CheckpointerPod) bool {
	if pod.Namespace != cp.PodNamespace {
		return false
	}
	return pod.Name == strings.TrimSuffix(cp.PodName, "-"+cp.NodeName)
}

func podListToParentPods(pl *v1.PodList) map[string]*v1.Pod {
	return podListToMap(pl, isValidParent)
}

func filterNone(p *v1.Pod) bool {
	return true
}

type filterFn func(*v1.Pod) bool

func podListToMap(pl *v1.PodList, filter filterFn) map[string]*v1.Pod {
	pods := make(map[string]*v1.Pod)
	for i := range pl.Items {
		if !filter(&pl.Items[i]) {
			continue
		}

		pod := &pl.Items[i]
		id := podFullName(pod)

		if _, ok := pods[id]; ok { // TODO(aaron): likely not be necessary (shouldn't ever happen) - but sanity check
			glog.Warningf("Found multiple local parent pods with same id: %s", id)
		}

		// Pods from Kubelet API do not have TypeMeta populated - set it here either way.
		pods[id] = pod
		pods[id].TypeMeta = metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		}
	}
	return pods
}

// A valid checkpoint parent:
//    has the checkpoint=true annotation
//    is not a static pod itself
//    is not a checkpoint pod itself
func isValidParent(pod *v1.Pod) bool {
	if pod.Annotations == nil {
		return false
	}
	shouldCheckpoint := pod.Annotations[shouldCheckpointAnnotation] == shouldCheckpoint
	isStatic := pod.Annotations[podSourceAnnotation] == podSourceFile
	return shouldCheckpoint && !isStatic && !isCheckpoint(pod)
}

func isCheckpoint(pod *v1.Pod) bool {
	if pod.Annotations == nil {
		return false
	}
	_, ok := pod.Annotations[checkpointParentAnnotation]
	return ok
}

func podFullName(pod *v1.Pod) string {
	return pod.Namespace + "/" + pod.Name
}

func podFullNameToInactiveCheckpointPath(id string) string {
	return filepath.Join(inactiveCheckpointPath, strings.Replace(id, "/", "-", -1)+".json")
}

func podFullNameToActiveCheckpointPath(id string) string {
	return filepath.Join(activeCheckpointPath, strings.Replace(id, "/", "-", -1)+".json")
}

// ErrorConflictingSecurityContexts is returned when a pod has a PodSecurityContext and/or
// SecurityContext(s) that have conflicting RunAsUser values.
var ErrorConflictingSecurityContexts = errors.New("pod and/or container(s) have conflicting SecurityContext.RunAsUser values")

// podUserAndGroup returns the ids of the user and group for the pod by scanning the
// PodSecurityContext and the SecurityContexts of its Containers. Returns
// ErrorConflictingSecurityContexts if the pod and/or its containers have different users/groups
// set.
func podUserAndGroup(pod *v1.Pod) (int, int, error) {
	var uid, gid *int64

	// Check PodSecurityContext.
	if psc := pod.Spec.SecurityContext; psc != nil {
		uid = psc.RunAsUser
		gid = psc.FSGroup
	}

	// Check Container SecurityContexts. If there is a conflict return error.
	// TODO(diegs): maybe resolve conflicts by returning per-container uids.
	for _, c := range pod.Spec.Containers {
		if sc := c.SecurityContext; sc != nil {
			if sc.RunAsUser != nil {
				// Fail if a different user was previously seen.
				if uid != nil && *uid != *sc.RunAsUser {
					return -1, -1, ErrorConflictingSecurityContexts
				}
				uid = sc.RunAsUser
			}
		}
	}

	// Return root uid/gid by default.
	if uid == nil {
		tmpUID := int64(rootUID)
		uid = &tmpUID
	}
	if gid == nil {
		tmpGID := int64(rootGID)
		gid = &tmpGID
	}

	return int(*uid), int(*gid), nil
}
