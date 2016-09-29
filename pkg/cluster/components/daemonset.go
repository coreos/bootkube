package components

import (
	"fmt"
	"strconv"
	"time"

	"github.com/golang/glog"
	"github.com/kubernetes-incubator/bootkube/pkg/cluster/components/version"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/cache"
	unversionedclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util/deployment"
	"k8s.io/kubernetes/pkg/util/wait"
)

const (
	daemonsetPodUpdateTimeout = 2 * time.Minute
)

// DaemonSetUpdater can provide rolling updates on
// the given DaemonSet.
type DaemonSetUpdater struct {
	// name of the DaemonSet object.
	name string
	// client is an API Server client.
	client unversionedclient.Interface
	// pods is backed by an informer, contains list of all Pods
	// in the kube-system namespace. Can be queried using the
	// DaemonSet selector to get pods for this DaemonSet.
	pods StoreToPodLister
	// daemonsets is a cache of DaemonSets backed by an informer.
	daemonsets cache.StoreToDaemonSetLister
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
		return nil, version.NoAnnotationError("DaemonSet", ds.Name)
	}
	ps, ok := ds.Annotations[version.UpdatePriorityAnnotation]
	if !ok {
		return nil, version.NoAnnotationError("DaemonSet", ds.Name)
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
	}, nil
}

// Name returns the name of the DaemonSet this updater
// is responsible for.
func (dsu *DaemonSetUpdater) Name() string {
	return dsu.name
}

// Version returns the highest version of any Pod managed
// by this DaemonSet.
func (dsu *DaemonSetUpdater) Version() (*version.Version, error) {
	dsi, exists, err := dsu.daemonsets.GetByKey(fmt.Sprintf("%s/%s", api.NamespaceSystem, dsu.Name()))
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("could not find a version for DaemonSet %s: does not exist in local store", dsu.Name())
	}
	ds, ok := dsi.(*extensions.DaemonSet)
	if !ok {
		return nil, fmt.Errorf("unexpected type returned from DaemonSet store, expected *extensions.DaemonSet, got %T", ds)
	}
	for _, c := range ds.Spec.Template.Spec.Containers {
		if c.Name == dsu.Name() {
			return version.ParseFromImageString(c.Image)
		}
	}
	return nil, fmt.Errorf("could not find a version for DaemonSet %s: could not find matching container", dsu.Name())
}

// Priority is the priority of updating this component.
func (dsu *DaemonSetUpdater) Priority() int {
	return dsu.priority
}

// UpdateToVersion will update the DaemonSet to the given version.
func (dsu *DaemonSetUpdater) UpdateToVersion(v *version.Version) (bool, error) {
	ds, err := dsu.client.Extensions().DaemonSets(api.NamespaceSystem).Get(dsu.Name())
	if err != nil {
		return false, err
	}
	// Create new DS.
	for i, c := range ds.Spec.Template.Spec.Containers {
		if c.Name == dsu.Name() {
			glog.Infof("updating image for container: %s", c.Name)
			if ds.Spec.Template.Spec.Containers[i].Image != v.ImageString() {
				ds.Spec.Template.Spec.Containers[i].Image = v.ImageString()
				_, err = dsu.client.Extensions().DaemonSets(api.NamespaceSystem).Update(ds)
				if err != nil {
					return false, fmt.Errorf("unable to update DaemonSet %s: %v\n\n%#v", dsu.Name(), err, ds)
				}
			}
			break
		}
	}
	pods, err := dsu.getPods()
	if err != nil {
		return false, err
	}
	updated := false
	glog.Infof("updating %d Pods for DaemonSet %s", len(pods), dsu.Name())
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
		err = wait.Poll(time.Second, daemonsetPodUpdateTimeout, func() (bool, error) {
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
				glog.Infof("%d desired pods and %d running, moving on", dsu.numberOfDesiredPods(), len(pl))
				return true, nil
			}

			glog.Infof("%d desired pods and %d running, will check again", dsu.numberOfDesiredPods(), len(pl))
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

func getPodVersion(pod *api.Pod, dsName string) (*version.Version, error) {
	for _, c := range pod.Spec.Containers {
		if c.Name == dsName {
			return version.ParseFromImageString(c.Image)
		}
	}
	return nil, fmt.Errorf("unable to get current version for Pod %s", pod.Name)
}
