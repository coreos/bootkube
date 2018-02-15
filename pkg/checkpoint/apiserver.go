package checkpoint

import (
	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
)

// getAPIParentPods will retrieve all pods from apiserver that are parents & should be checkpointed
// Returns false if we could not contact the apiserver.
func (c *checkpointer) getAPIParentPods(nodeName string) (bool, map[string]*v1.Pod) {
	opts := metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("spec.nodeName", nodeName).String(),
	}

	podList, err := c.apiserver.CoreV1().Pods(c.checkpointerPod.PodNamespace).List(opts)
	if err != nil {
		glog.Warningf("Unable to contact APIServer, skipping garbage collection: %v", err)
		return false, nil
	}
	return true, podListToParentPods(podList)
}
