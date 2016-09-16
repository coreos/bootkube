package components

import (
	"fmt"
	"strconv"
	"time"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/cache"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/release_1_3"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util/deployment"
	"k8s.io/kubernetes/pkg/util/wait"
)

// DaemonSetUpdater can provide rolling updates on
// the given DaemonSet.
type DaemonSetUpdater struct {
	// name of the DaemonSet object.
	name string
	// client is an API Server client.
	client clientset.Interface
	// pods is backed by an informer, contains list of Pods
	// that belong to the DaemonSet.
	pods StoreToPodLister
	// selector is the DaemonSet selector.
	selector labels.Selector
	// priority is the update priority for this DaemonSet.
	priority int
}

// StoreToPodLister is a mirror of the cache.StoreToPodLister.
// The main difference is this accepts a cache.Store instead of
// a cache.Indexer.
type StoreToPodLister struct {
	cache.Store
}

func (s *StoreToPodLister) List(selector labels.Selector) (pods []*api.Pod, err error) {
	for _, m := range s.Store.List() {
		pod := m.(*api.Pod)
		if selector.Matches(labels.Set(pod.Labels)) {
			pods = append(pods, pod)
		}
	}
	return pods, nil
}

func NewDaemonSetUpdater(client clientset.Interface, ds *extensions.DaemonSet, pods StoreToPodLister) (*DaemonSetUpdater, error) {
	selector, err := unversioned.LabelSelectorAsSelector(ds.Spec.Selector)
	if err != nil {
		return nil, err
	}
	if ds.Annotations == nil {
		return nil, noAnnotationError("DaemonSet", ds.Name)
	}
	ps, ok := ds.Annotations[updatePriorityAnnotation]
	if !ok {
		return nil, noAnnotationError("DaemonSet", ds.Name)
	}
	priority, err := strconv.Atoi(ps)
	if err != nil {
		return nil, err
	}
	return &DaemonSetUpdater{
		name:     ds.Name,
		client:   client,
		pods:     pods,
		selector: selector,
		priority: priority,
	}, nil
}

// Name returns the name of the DaemonSet this updater
// is responsible for.
func (dsu *DaemonSetUpdater) Name() string {
	return dsu.name
}

// Priority is the priority of updating this component.
func (dsu *DaemonSetUpdater) Priority() int {
	return dsu.priority
}

// UpdateToVersion will update the DaemonSet to the given version.
func (dsu *DaemonSetUpdater) UpdateToVersion(v *Version) error {
	ds, err := dsu.client.Extensions().DaemonSets(api.NamespaceSystem).Get(dsu.Name())
	if err != nil {
		return err
	}
	// Create new DS.
	ds.Labels["version"] = v.Image.Tag
	ds.Spec.Template.Labels["version"] = v.Image.Tag
	for i, c := range ds.Spec.Template.Spec.Containers {
		if c.Name == dsu.Name() {
			glog.Infof("updating image for container: %s", c.Name)
			ds.Spec.Template.Spec.Containers[i].Image = v.Image.String()
			break
		}
	}
	ds, err = dsu.client.Extensions().DaemonSets(api.NamespaceSystem).Update(ds)
	if err != nil {
		return err
	}
	pods, err := dsu.getPods()
	if err != nil {
		return err
	}
	for i, p := range pods {
		pv, err := getPodVersion(p)
		if err != nil {
			return fmt.Errorf("unable to update DaemonSet %s: %#v", dsu.Name(), err)
		}
		if pv.Semver.EQ(v.Semver) {
			continue
		}
		// Delete old DS Pod.
		glog.Infof("Deleting pod %s", p.Name)
		err = dsu.client.Core().Pods(api.NamespaceSystem).Delete(p.Name, nil)
		if err != nil {
			return err
		}
		glog.Infof("Deleted pod %s", p.Name)

		// Wait for all pods to be available before moving on.
		err = wait.Poll(time.Second, 10*time.Minute, func() (bool, error) {
			glog.Infof("checking new pod availability for DS: %s", dsu.Name)

			updatedPodCount := i + 1
			if dsu.podsRunningNewVersion(v) == updatedPodCount && dsu.allPodsAvailable() {
				return true, nil
			}

			glog.Info("Pod not ready, will check again")
			return false, nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (dsu *DaemonSetUpdater) getPods() ([]*api.Pod, error) {
	return dsu.pods.List(dsu.selector)
}

func (dsu *DaemonSetUpdater) allPodsAvailable() bool {
	pods, err := dsu.getPods()
	if err != nil {
		return false
	}
	for _, p := range pods {
		available := deployment.IsPodAvailable(p, 5)
		if !available {
			return false
		}
	}
	return true
}

func (dsu *DaemonSetUpdater) podsRunningNewVersion(v *Version) int {
	pods, err := dsu.getPods()
	if err != nil {
		return 0
	}

	count := 0
	for _, p := range pods {
		for _, c := range p.Spec.Containers {
			if c.Image == v.Image.String() {
				count++
				break
			}
		}
	}
	return count
}

func getPodVersion(pod *api.Pod) (*Version, error) {
	var v *Version
	for _, c := range pod.Spec.Containers {
		if c.Name == pod.Name {
			ver, err := ParseVersionFromImage(c.Image)
			if err != nil {
				return nil, err
			}
			if v == nil {
				v = ver
			} else if v.Semver.GT(ver.Semver) {
				v = ver
			}
			break
		}
	}
	if v == nil {
		return nil, fmt.Errorf("unable to get current version for Pod %s", pod.Name)
	}
	return v, nil
}
