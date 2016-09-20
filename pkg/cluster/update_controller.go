package cluster

import (
	"fmt"
	"sort"
	"time"

	"github.com/kubernetes-incubator/bootkube/pkg/cluster/components"
	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
	"k8s.io/kubernetes/pkg/client/cache"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/release_1_3"
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
)

type NonNodeComponentsGetterFn func(clientset.Interface, cache.StoreToDaemonSetLister, cache.StoreToDeploymentLister, components.StoreToPodLister, cache.StoreToNodeLister) ([]Component, error)

type NodeComponenetsGetterFn func(clientset.Interface, cache.StoreToNodeLister) ([]Component, error)

// UpdateController is responsible for safely updating an entire cluster.
type UpdateController struct {
	// Client is a generic API server client.
	Client clientset.Interface
	// AllNonNodeManagedComponentsFn is a function that should return
	// a list of every non-Node component the update controller is managing.
	GetAllNonNodeManagedComponentsFn NonNodeComponentsGetterFn

	// AllManagedNodesFn should return a list of every managed Node in the cluster.
	GetAllManagedNodesFn NodeComponenetsGetterFn

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
	UpdateToVersion(*components.Version) (bool, error)
	// Priority is the priority level for this component.
	Priority() int
	// Version of the component.
	Version() (*components.Version, error)
}

// NewClusterUpdater returns a ClusterUpdater struct with defaults.
func NewClusterUpdater(client clientset.Interface) (*UpdateController, error) {
	l, err := labels.Parse(clusterManagedLabel)
	if err != nil {
		return nil, err
	}
	mlo := api.ListOptions{LabelSelector: l}
	nodeStore, nodeController := framework.NewInformer(
		&cache.ListWatch{
			ListFunc: func(_ api.ListOptions) (runtime.Object, error) {
				return client.Core().Nodes().List(mlo)
			},
			WatchFunc: func(_ api.ListOptions) (watch.Interface, error) {
				return client.Core().Nodes().Watch(mlo)
			},
		},
		&v1.Node{},
		30*time.Minute,
		framework.ResourceEventHandlerFuncs{},
	)
	daemonSetStore, daemonSetController := framework.NewInformer(
		&cache.ListWatch{
			ListFunc: func(_ api.ListOptions) (runtime.Object, error) {
				return client.Extensions().DaemonSets(api.NamespaceSystem).List(mlo)
			},
			WatchFunc: func(_ api.ListOptions) (watch.Interface, error) {
				return client.Extensions().DaemonSets(api.NamespaceSystem).Watch(mlo)
			},
		},
		&v1beta1.DaemonSet{},
		30*time.Minute,
		framework.ResourceEventHandlerFuncs{},
	)
	deploymentStore, deploymentController := framework.NewInformer(
		&cache.ListWatch{
			ListFunc: func(_ api.ListOptions) (runtime.Object, error) {
				return client.Extensions().Deployments(api.NamespaceSystem).List(mlo)
			},
			WatchFunc: func(_ api.ListOptions) (watch.Interface, error) {
				return client.Extensions().Deployments(api.NamespaceSystem).Watch(mlo)
			},
		},
		&v1beta1.Deployment{},
		30*time.Minute,
		framework.ResourceEventHandlerFuncs{},
	)
	podStore, podController := framework.NewInformer(
		&cache.ListWatch{
			ListFunc: func(lo api.ListOptions) (runtime.Object, error) {
				return client.Core().Pods(api.NamespaceSystem).List(lo)
			},
			WatchFunc: func(lo api.ListOptions) (watch.Interface, error) {
				return client.Core().Pods(api.NamespaceSystem).Watch(lo)
			},
		},
		&v1.Pod{},
		30*time.Minute,
		framework.ResourceEventHandlerFuncs{},
	)

	go nodeController.Run(wait.NeverStop)
	go daemonSetController.Run(wait.NeverStop)
	go deploymentController.Run(wait.NeverStop)
	go podController.Run(wait.NeverStop)

	return &UpdateController{
		Client: client,
		GetAllNonNodeManagedComponentsFn: DefaultGetAllManagedNonNodeComponentsFn,
		GetAllManagedNodesFn:             DefaultGetManagedNodeComponentsFn,
		nodes:                            cache.StoreToNodeLister{nodeStore},
		deployments:                      cache.StoreToDeploymentLister{deploymentStore},
		daemonSets:                       cache.StoreToDaemonSetLister{daemonSetStore},
		pods:                             components.StoreToPodLister{podStore},
	}, nil
}

// UpdateToVersion will update the cluster to the given version.
func (cu *UpdateController) UpdateToVersion(v *components.Version) error {
	comps, err := cu.GetAllNonNodeManagedComponentsFn(
		cu.Client,
		cu.daemonSets,
		cu.deployments,
		cu.pods,
		cu.nodes,
	)
	if err != nil {
		return err
	}
	hv, err := highestClusterVersion(comps)
	if err != nil {
		return err
	}
	comps = sortComponentsByPriority(hv, v, comps)
	nodeComps, err := cu.GetAllManagedNodesFn(cu.Client, cu.nodes)
	if err != nil {
		return err
	}
	comps = append(comps, nodeComps...)
	for _, c := range comps {
		glog.Infof("Begin update of component: %s", c.Name())
		updated, err := c.UpdateToVersion(v)
		if err != nil {
			err = fmt.Errorf("Failed update of component: %s due to: %v", c.Name(), err)
			glog.Error(err)
			return err
		}
		glog.Infof("Finished update of componenet: %s", c.Name())
		// Return once we've updated a component and then re-check the list the next
		// time around. This ensures that we're always keeping every component at
		// the correct version, even if they are updated out-of-band during the
		// course of an upgrade.
		if updated {
			return nil
		}
	}
	return nil
}

func DefaultGetAllManagedNonNodeComponentsFn(client clientset.Interface,
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
		dsu, err := components.NewDaemonSetUpdater(client, &ds, daemonsets, pods)
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
		du, err := components.NewDeploymentUpdater(client, &dp)
		if err != nil {
			return nil, err
		}
		comps = append(comps, du)
	}
	return comps, nil
}

func DefaultGetManagedNodeComponentsFn(client clientset.Interface, nodes cache.StoreToNodeLister) ([]Component, error) {
	nl, err := nodes.List()
	if err != nil {
		return nil, err
	}
	var comps []Component
	for _, n := range nl.Items {
		nu, err := components.NewNodeUpdater(client, &n, nodes)
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

// We need to sort in asc/desc order based on the version skew.
//
// For example:
//
// If the version is higher than the highest versioned component
// in the cluster, then we execute the update in ascending priority.
//
// If the version is lower than the highest versioned component
// in the cluster, then we execute the update in descending priority.
func sortComponentsByPriority(highestClusterVersion, newVersion *components.Version, comps []Component) []Component {
	if newVersion.Semver().GT(highestClusterVersion.Semver()) {
		glog.Info("sorting components by ascending priority")
		sort.Sort(byAscendingPriority(comps))
	} else {
		glog.Info("sorting components by descending priority")
		sort.Sort(byDescendingPriority(comps))
	}
	return comps
}

func highestClusterVersion(comps []Component) (*components.Version, error) {
	var highestVersion *components.Version
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
	return highestVersion, nil
}
