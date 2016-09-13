package cluster

import (
	"fmt"
	"os"

	"github.com/coreos/bootkube/pkg/cluster/components"
	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/api"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/release_1_3"
	"k8s.io/kubernetes/pkg/client/record"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/runtime"
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
	clusterManagedAnnotation = "metadata.annotations.alpha.coreos.com/cluster-updater/managed"
)

// ClusterUpdater is responsible for safely updating an entire cluster.
type ClusterUpdater struct {
	// Client is a generic API server client.
	Client clientset.Interface
	// Components is a slice of components to be updated.
	// Each will be updated in the order they appear in the
	// slice.
	Components []ComponentUpdater
	// Events is used to send events during the cluster update.
	Events record.EventRecorder
	// SelfObj is the runtime object for ourself to use for events.
	SelfObj runtime.Object
}

// ComponentUpdater is responsible for updating
// a single component in the cluster.
// It takes the name of the component and a function
// that will be used to update that component.
// The name should be the name of the component
// as it appears in the manifest file.
type ComponentUpdater interface {
	// Name is the name of the component to update.
	Name() string
	// UpdateToVersion is the function used to update this component to the
	// provided version.
	UpdateToVersion(clientset.Interface, *components.Version) error
	// CurrentVersion is the current version of this component.
	CurrentVersion() (*components.Version, error)
}

// NewClusterUpdater returns a ClusterUpdater struct with defaults.
func NewClusterUpdater(client clientset.Interface) (*ClusterUpdater, error) {
	b := record.NewBroadcaster()
	r := b.NewRecorder(api.EventSource{
		Component: "cluster-updater",
	})
	b.StartRecordingToSink(client.Core().Events(api.NamespaceSystem).(unversioned.EventInterface))
	p, err := client.Core().Pods(api.NamespaceSystem).Get(os.Getenv("POD_NAME"))
	if err != nil {
		return nil, fmt.Errorf("error getting self pod: %v", err)
	}
	return &ClusterUpdater{
		Client:  client,
		Events:  r,
		SelfObj: p,
	}, nil
}

// UpdateToVersion will update the cluster to the given version.
func (cu *ClusterUpdater) UpdateToVersion(v *components.Version) error {
	comps, err := getManagedComponentList(cu.Client)
	if err != nil {
		return err
	}
	for _, c := range comps {
		cv, err := c.CurrentVersion()
		if err != nil {
			return err
		}
		if cv.Semver.LT(v.Semver) {
			cu.Events.Eventf(cu.SelfObj, api.EventTypeNormal, "Begin update of component: %s", c.Name())
			glog.Infof("Begin update of component: %s", c.Name())
			if err := c.UpdateToVersion(cu.Client, v); err != nil {
				cu.Events.Eventf(cu.SelfObj, api.EventTypeWarning,
					"Failed update of component: %s due to: %v",
					c.Name(), err)
				glog.Errorf("Failed update of component: %s due to: %v",
					c.Name(), err)
				return err
			}
			glog.Infof("Finshed update of componenet: %s", c.Name)
			cu.Events.Eventf(cu.SelfObj, api.EventTypeNormal, "Finished update of component: %s", c.Name())
		}
	}
	return nil
}

func getManagedComponentList(client clientset.Interface) ([]ComponentUpdater, error) {
	var comps []ComponentUpdater
	lo := api.ListOptions{FieldSelector: fields.OneTermEqualSelector(clusterManagedAnnotation, "true")}
	// Get DaemonSets
	dsl, err := client.Extensions().DaemonSets(api.NamespaceSystem).List(lo)
	if err != nil {
		return nil, err
	}
	for _, ds := range dsl.Items {
		dsu, err := components.NewDaemonSetUpdater(client, &ds)
		if err != nil {
			return nil, err
		}
		comps = append(comps, dsu)
	}
	// Get Deployments
	dpl, err := client.Extensions().Deployments(api.NamespaceSystem).List(lo)
	if err != nil {
		return nil, err
	}
	for _, dp := range dpl.Items {
		comps = append(comps, components.NewDeploymentUpdater(client, &dp))
	}
	// Get Nodes
	nu, err := components.NewNodeUpdater(client, lo)
	if err != nil {
		return nil, err
	}
	return append(comps, nu), nil
}
