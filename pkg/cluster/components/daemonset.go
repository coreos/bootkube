package components

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
	"k8s.io/kubernetes/pkg/client/cache"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/release_1_3"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/deployment"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/watch"
)

// DaemonSetUpdater can provide rolling updates on
// the given DaemonSet.
type DaemonSetUpdater struct {
	// name of the DaemonSet object.
	name string
	// client is an API Server client.
	client clientset.Interface
	// podStore is backed by an informer, contains list of Pods
	// that belong to the DaemonSet.
	podStore cache.Store
}

func NewDaemonSetUpdater(client clientset.Interface, ds *v1beta1.DaemonSet) (*DaemonSetUpdater, error) {
	var spec extensions.DaemonSetSpec
	err := v1beta1.Convert_v1beta1_DaemonSetSpec_To_extensions_DaemonSetSpec(&ds.Spec, &spec, nil)
	if err != nil {
		return nil, err
	}
	selector, err := unversioned.LabelSelectorAsSelector(spec.Selector)
	if err != nil {
		return nil, err
	}
	lo := api.ListOptions{LabelSelector: selector}
	podStore, podController := framework.NewInformer(
		&cache.ListWatch{
			ListFunc: func(_ api.ListOptions) (runtime.Object, error) {
				return client.Core().Pods(ds.Namespace).List(lo)
			},
			WatchFunc: func(_ api.ListOptions) (watch.Interface, error) {
				return client.Core().Pods(ds.Namespace).Watch(lo)
			},
		},
		&v1.Pod{},
		30*time.Minute,
		framework.ResourceEventHandlerFuncs{},
	)
	go podController.Run(wait.NeverStop)
	return &DaemonSetUpdater{
		name:     ds.Name,
		client:   client,
		podStore: podStore,
	}, nil
}

// Name returns the name of the DaemonSet this updater
// is responsible for.
func (dsu *DaemonSetUpdater) Name() string {
	return dsu.name
}

// CurrentVersion is the lowest version of any Pod managed by
// the DaemonSet. Using the lowest version ensures that
// if we failed to update all Pods in the DaemonSet, it
// will be attempted again next time around.
func (dsu *DaemonSetUpdater) CurrentVersion() (*Version, error) {
	var v *Version
	for _, pi := range dsu.podStore.List() {
		p := pi.(*v1.Pod)
		for _, c := range p.Spec.Containers {
			if c.Name == p.Name {
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
	}
	if v == nil {
		return nil, fmt.Errorf("unable to get current version for DaemonSet %s", dsu.Name())
	}
	return v, nil
}

// UpdateToVersion will update the DaemonSet to the given version.
func (dsu *DaemonSetUpdater) UpdateToVersion(client clientset.Interface, v *Version) error {
	ds, err := client.Extensions().DaemonSets(api.NamespaceSystem).Get(dsu.Name())
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
	pods := dsu.podStore.List()
	for i, pi := range pods {
		p := pi.(*v1.Pod)
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
			if podsRunningNewVersion(dsu.podStore, v) == updatedPodCount && allPodsAvailable(dsu.podStore) {
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

func allPodsAvailable(podStore cache.Store) bool {
	pl := podStore.List()
	for _, pi := range pl {
		p := pi.(*v1.Pod)
		var apiPod api.Pod
		v1.Convert_v1_Pod_To_api_Pod(p, &apiPod, nil)
		available := deployment.IsPodAvailable(&apiPod, 5)
		if !available {
			return false
		}
	}
	return true
}

func podsRunningNewVersion(podStore cache.Store, v *Version) int {
	count := 0
	pl := podStore.List()
	for _, pi := range pl {
		p := pi.(*v1.Pod)
		for _, c := range p.Spec.Containers {
			if c.Image == v.Image.String() {
				count++
				break
			}
		}
	}
	return count
}
