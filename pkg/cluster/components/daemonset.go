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
	unversionedclient "k8s.io/kubernetes/pkg/client/unversioned"
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
	client unversionedclient.Interface
	// pods is backed by an informer, contains list of Pods
	// that belong to the DaemonSet.
	pods StoreToPodLister
	// daemonsets is a cache of DaemonSets backed by an informer.
	daemonsets cache.StoreToDaemonSetLister
	// selector is the DaemonSet selector.
	selector labels.Selector
	// priority is the update priority for this DaemonSet.
	priority int
	// obj is the DaemonSet Object.
	obj *extensions.DaemonSet
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

// Exists returns true if a pod matching the namespace/name of the given pod exists in the store.
func (s *StoreToPodLister) Exists(pod *api.Pod) (bool, error) {
	_, exists, err := s.Store.Get(pod)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func NewDaemonSetUpdater(client unversionedclient.Interface, ds *extensions.DaemonSet, daemonsets cache.StoreToDaemonSetLister, pods StoreToPodLister) (*DaemonSetUpdater, error) {
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
		name:       ds.Name,
		client:     client,
		pods:       pods,
		daemonsets: daemonsets,
		selector:   selector,
		priority:   priority,
		obj:        ds,
	}, nil
}

// Name returns the name of the DaemonSet this updater
// is responsible for.
func (dsu *DaemonSetUpdater) Name() string {
	return dsu.name
}

// Version returns the highest version of any Pod managed
// by this DaemonSet.
func (dsu *DaemonSetUpdater) Version() (*Version, error) {
	pods, err := dsu.getPods()
	if err != nil {
		return nil, err
	}
	var highest *Version
	for _, p := range pods {
		pv, err := getPodVersion(p, dsu.Name())
		if err != nil {
			return nil, fmt.Errorf("unable to get Pod %s Version: %v", p.Name, err)
		}
		if highest == nil {
			highest = pv
			continue
		}
		if pv.Semver().GT(highest.Semver()) {
			highest = pv
		}
	}
	if highest == nil {
		return nil, fmt.Errorf("could not find a version for DaemonSet %s", dsu.Name())
	}
	return highest, nil
}

// Priority is the priority of updating this component.
func (dsu *DaemonSetUpdater) Priority() int {
	return dsu.priority
}

// UpdateToVersion will update the DaemonSet to the given version.
func (dsu *DaemonSetUpdater) UpdateToVersion(v *Version) (bool, error) {
	ds, err := dsu.client.Extensions().DaemonSets(api.NamespaceSystem).Get(dsu.Name())
	if err != nil {
		return false, err
	}
	// Create new DS.
	for i, c := range ds.Spec.Template.Spec.Containers {
		if c.Name == dsu.Name() {
			glog.Infof("updating image for container: %s", c.Name)
			ds.Spec.Template.Spec.Containers[i].Image = v.image.String()
			break
		}
	}
	ds, err = dsu.client.Extensions().DaemonSets(api.NamespaceSystem).Update(ds)
	if err != nil {
		return false, fmt.Errorf("unable to update DaemonSet %s: %v\n\n%#v", dsu.Name(), err, ds)
	}
	pods, err := dsu.getPods()
	if err != nil {
		return false, err
	}
	updated := false
	for _, p := range pods {
		pv, err := getPodVersion(p, dsu.Name())
		if err != nil {
			return false, fmt.Errorf("unable to get Pod %s Version: %#v", p.Name, err)
		}
		// If this Pod has already been updated, skip it.
		if pv.Semver().EQ(v.Semver()) {
			continue
		}
		updated = true
		// Delete old DS Pod.
		glog.Infof("Deleting pod %s", p.Name)
		err = dsu.client.Pods(api.NamespaceSystem).Delete(p.Name, nil)
		if err != nil {
			return false, err
		}
		glog.Infof("Deleted pod %s", p.Name)

		// Wait for all pods to be available before moving on.
		err = wait.Poll(time.Second, 2*time.Minute, func() (bool, error) {
			glog.Infof("checking new pod availability for DS: %s", dsu.Name())

			// Make sure the pod we deleted above is removed from the cache.
			exists, err := dsu.pods.Exists(p)
			if exists {
				return false, err
			}

			pl, err := dsu.getPods()
			if err != nil {
				return false, nil
			}

			if dsu.numberOfDesiredPods() == len(pl) && dsu.allPodsAvailable() {
				glog.Info("Pod ready, moving on")
				return true, nil
			}

			glog.Info("Pod not ready, will check again")
			return false, nil
		})
		if err != nil {
			return false, err
		}
	}
	return updated, nil
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

func (dsu *DaemonSetUpdater) numberOfDesiredPods() int {
	dsl, err := dsu.daemonsets.List()
	if err != nil {
		return 0
	}
	for _, ds := range dsl.Items {
		if ds.Name == dsu.Name() {
			return int(ds.Status.DesiredNumberScheduled)
		}
	}
	return 0
}

func getPodVersion(pod *api.Pod, dsName string) (*Version, error) {
	for _, c := range pod.Spec.Containers {
		if c.Name == dsName {
			return ParseVersionFromImage(c.Image)
		}
	}
	return nil, fmt.Errorf("unable to get current version for Pod %s", pod.Name)
}
