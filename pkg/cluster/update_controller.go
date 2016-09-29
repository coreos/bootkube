package cluster

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/golang/glog"

	"github.com/kubernetes-incubator/bootkube/pkg/cluster/components"
	"github.com/kubernetes-incubator/bootkube/pkg/cluster/components/version"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/watch"
)

const (
	// ClusterConfigMapName is the name of the config map that holds cluster configuration,
	// including the cluster version to run.
	ClusterConfigMapName = "cluster-config"
	// ClusterVersionKey is the key in the cluster ConfigMap that holds the version of the cluster
	// that should be running.
	ClusterVersionKey = "cluster.version"
	// clusterMangedAnnotation is the annotation used to denote a managed component within
	// the cluster.
	clusterManagedLabel = "update-controller-managed"

	informerResyncPeriod = 2 * time.Minute
)

type ComponentsGetterFn func(unversioned.Interface, internalclientset.Interface, cache.StoreToDaemonSetLister, cache.StoreToDeploymentLister, components.StoreToPodLister, cache.StoreToNodeLister) ([]Component, error)

// UpdateController is responsible for safely updating an entire cluster.
type UpdateController struct {
	// Client is an API Server client.
	Client unversioned.Interface
	// InternalClient is another client type, used for
	// certain functions.
	InternalClient internalclientset.Interface
	// AllNonNodeManagedComponentsFn is a function that should return
	// a list of every non-Node component the update controller is managing.
	GetAllManagedComponentsFn ComponentsGetterFn

	configmaps cache.Store

	// These stores hold all of the managed components.
	nodes       cache.StoreToNodeLister
	deployments cache.StoreToDeploymentLister
	daemonSets  cache.StoreToDaemonSetLister

	// pods managed by DaemonSets. Allows lookup by DaemonSet selector.
	pods components.StoreToPodLister
}

// Component is responsible for updating
// a single component in the cluster.
// It takes the name of the component and a function
// that will be used to update that component.
// The name should be the name of the component
// as it appears in the manifest file.
type Component interface {
	// Name is the name of the component to update.
	Name() string
	// UpdateToVersion is the function used to update this component to the
	// provided version.
	UpdateToVersion(*version.Version) (bool, error)
	// Priority is the priority level for this component.
	Priority() int
	// Version of the component.
	Version() (*version.Version, error)
}

// NewUpdateController returns a UpdateController with defaults.
func NewUpdateController(uc unversioned.Interface, internalclient internalclientset.Interface, configMapStore cache.Store) (*UpdateController, error) {
	l, err := labels.Parse(clusterManagedLabel)
	if err != nil {
		return nil, err
	}
	mlo := api.ListOptions{LabelSelector: l}
	nodeStore, nodeController := framework.NewInformer(
		&cache.ListWatch{
			ListFunc: func(_ api.ListOptions) (runtime.Object, error) {
				return uc.Nodes().List(mlo)
			},
			WatchFunc: func(_ api.ListOptions) (watch.Interface, error) {
				return uc.Nodes().Watch(mlo)
			},
		},
		&api.Node{},
		informerResyncPeriod,
		framework.ResourceEventHandlerFuncs{},
	)
	daemonSetStore, daemonSetController := framework.NewInformer(
		&cache.ListWatch{
			ListFunc: func(_ api.ListOptions) (runtime.Object, error) {
				return uc.Extensions().DaemonSets(api.NamespaceSystem).List(mlo)
			},
			WatchFunc: func(_ api.ListOptions) (watch.Interface, error) {
				return uc.Extensions().DaemonSets(api.NamespaceSystem).Watch(mlo)
			},
		},
		&extensions.DaemonSet{},
		informerResyncPeriod,
		framework.ResourceEventHandlerFuncs{},
	)
	deploymentStore, deploymentController := framework.NewInformer(
		&cache.ListWatch{
			ListFunc: func(_ api.ListOptions) (runtime.Object, error) {
				return uc.Extensions().Deployments(api.NamespaceSystem).List(mlo)
			},
			WatchFunc: func(_ api.ListOptions) (watch.Interface, error) {
				return uc.Extensions().Deployments(api.NamespaceSystem).Watch(mlo)
			},
		},
		&extensions.Deployment{},
		informerResyncPeriod,
		framework.ResourceEventHandlerFuncs{},
	)
	podStore, podController := framework.NewInformer(
		&cache.ListWatch{
			ListFunc: func(lo api.ListOptions) (runtime.Object, error) {
				return uc.Pods(api.NamespaceSystem).List(lo)
			},
			WatchFunc: func(lo api.ListOptions) (watch.Interface, error) {
				return uc.Pods(api.NamespaceSystem).Watch(lo)
			},
		},
		&api.Pod{},
		informerResyncPeriod,
		framework.ResourceEventHandlerFuncs{},
	)

	go nodeController.Run(wait.NeverStop)
	go daemonSetController.Run(wait.NeverStop)
	go deploymentController.Run(wait.NeverStop)
	go podController.Run(wait.NeverStop)

	return &UpdateController{
		Client:                    uc,
		InternalClient:            internalclient,
		GetAllManagedComponentsFn: DefaultGetAllManagedComponentsFn,
		configmaps:                configMapStore,
		nodes:                     cache.StoreToNodeLister{nodeStore},
		deployments:               cache.StoreToDeploymentLister{deploymentStore},
		daemonSets:                cache.StoreToDaemonSetLister{daemonSetStore},
		pods:                      components.StoreToPodLister{podStore},
	}, nil
}

func (uc *UpdateController) Run(stop <-chan struct{}) {
	glog.Info("update-controller running")
	for {
		sleepDuration := time.Millisecond
		select {
		case <-stop:
			glog.Info("update-controller stopping...")
			return
		default:
			obj, exists, err := uc.configmaps.GetByKey(fmt.Sprintf("%s/%s", api.NamespaceSystem, ClusterConfigMapName))
			if err != nil {
				glog.Error(err)
				sleepDuration = time.Second
				break
			}
			if !exists {
				sleepDuration = time.Second
				break
			}
			cm, ok := obj.(*api.ConfigMap)
			if !ok {
				glog.Fatalf("Got unexpected type from ConfigMap store, expected *api.ConfigMap, got %T", obj)
				sleepDuration = time.Second
			}
			newVersion, err := parseNewVersion(cm)
			if err != nil {
				glog.Error(err)
				break
			}
			updated, err := uc.UpdateToVersion(newVersion)
			if err != nil {
				glog.Error(err)
			}
			if !updated {
				// There was nothing to be done, sleep for a bit and check again.
				sleepDuration = 30 * time.Second
			}
			time.Sleep(sleepDuration)
		}
	}
}

// UpdateToVersion will update the cluster to the given version.
func (uc *UpdateController) UpdateToVersion(v *version.Version) (bool, error) {
	comps, err := uc.GetAllManagedComponentsFn(
		uc.Client,
		uc.InternalClient,
		uc.daemonSets,
		uc.deployments,
		uc.pods,
		uc.nodes,
	)
	if err != nil {
		return false, err
	}
	// Get the highest cluster version, which allows us to determine
	// whether we are updating or rolling back.
	hv, err := highestClusterVersion(comps)
	if err != nil {
		return false, err
	}
	// Get a sorted list of components we should update. Each component
	// is sorted by priority corresponding to the desired version.
	// For more info on the sorting logic, see the documentation on `sortComponentsByPriority`.
	comps = sortComponentsByPriority(hv, v, comps)
	for _, c := range comps {
		glog.Infof("Begin update of component: %s", c.Name())
		updated, err := c.UpdateToVersion(v)
		if err != nil {
			return false, fmt.Errorf("Failed update of component: %s due to: %v", c.Name(), err)
		}
		// Return once we've updated a component and then re-check the list the next
		// time around. This ensures that we're always keeping every component at
		// the correct version, even if they are updated out-of-band during the
		// course of an upgrade.
		if updated {
			glog.Infof("Finished update of componenet: %s", c.Name())
			return true, nil
		}
		glog.Infof("Component %s already updated, moving on", c.Name())
	}
	return false, nil
}

func DefaultGetAllManagedComponentsFn(uc unversioned.Interface,
	internalclient internalclientset.Interface,
	daemonsets cache.StoreToDaemonSetLister,
	deployments cache.StoreToDeploymentLister,
	pods components.StoreToPodLister,
	nodes cache.StoreToNodeLister) ([]Component, error) {

	var comps []Component
	// Add DaemonSets
	dsl, err := daemonsets.List()
	if err != nil {
		return nil, err
	}
	for _, ds := range dsl.Items {
		dsu, err := components.NewDaemonSetUpdater(uc, &ds, daemonsets, pods)
		if err != nil {
			return nil, err
		}
		comps = append(comps, dsu)
	}
	// Add Deployments
	dpls, err := deployments.List()
	if err != nil {
		return nil, err
	}
	for _, dp := range dpls {
		du, err := components.NewDeploymentUpdater(uc, internalclient, &dp)
		if err != nil {
			return nil, err
		}
		comps = append(comps, du)
	}
	// Add Nodes
	nl, err := nodes.List()
	if err != nil {
		return nil, err
	}
	for _, n := range nl.Items {
		nu, err := components.NewNodeUpdater(uc, &n, nodes)
		if err != nil {
			return nil, err
		}
		comps = append(comps, nu)
	}
	return comps, nil
}

type byAscendingPriority []Component

func (a byAscendingPriority) Len() int           { return len(a) }
func (a byAscendingPriority) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byAscendingPriority) Less(i, j int) bool { return a[i].Priority() < a[j].Priority() }

type byDescendingPriority []Component

func (a byDescendingPriority) Len() int           { return len(a) }
func (a byDescendingPriority) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byDescendingPriority) Less(i, j int) bool { return a[i].Priority() > a[j].Priority() }

// Sort in asc/desc order based on the version skew.
//
// For example:
//
// If the desired version is higher than the highest versioned component
// in the cluster, then we execute the update in ascending priority.
//
// If the desired version is lower than the highest versioned component
// in the cluster, then we execute the update in descending priority.
//
// We sort this way because of certain requirements when updating a cluster.
// For example, we should always update the API Server first during a normal update
// so it should have the highest priority. During a rollback however, we should
// update that component last. Sorting based on the desired version, and the highest
// cluster version allows us to determine the correct order for updating components.
func sortComponentsByPriority(highestClusterVersion, newVersion *version.Version, comps []Component) []Component {
	if newVersion.Semver().GTE(highestClusterVersion.Semver()) {
		glog.Info("sorting components by ascending priority")
		sort.Sort(byAscendingPriority(comps))
	} else {
		glog.Info("sorting components by descending priority")
		sort.Sort(byDescendingPriority(comps))
	}
	return comps
}

// highestClusterVersion returns the highest version of any component running in the
// cluster. We consider the highest version running to be the current cluster version
// and use that information to determine component ordering during updates.
func highestClusterVersion(comps []Component) (*version.Version, error) {
	var highestVersion *version.Version
	for _, comp := range comps {
		cv, err := comp.Version()
		if err != nil {
			return nil, err
		}
		if highestVersion == nil {
			highestVersion = cv
			continue
		}
		if cv.Semver().GT(highestVersion.Semver()) {
			highestVersion = cv
		}
	}
	if highestVersion == nil {
		return nil, errors.New("unable to get highest cluster version")
	}
	return highestVersion, nil
}

func parseNewVersion(config *api.ConfigMap) (*version.Version, error) {
	return version.ParseFromImageString(config.Data[ClusterVersionKey])
}
