package cluster

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"time"

	"github.com/coreos/bootkube/pkg/node"
	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/api/v1"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/release_1_3"
	"k8s.io/kubernetes/pkg/client/record"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"
)

const (
	// ClusterConfigMapName is the name of the config map that holds cluster configuration,
	// including the cluster version to run.
	ClusterConfigMapName = "cluster-config"
	// ClusterVersionKey is the key in the cluster ConfigMap that holds the version of the cluster
	// that should be running.
	ClusterVersionKey = "cluster.version"
)

// DefaultComponentUpdaterList is the default order in which cluster components should
// be updated.
var DefaultComponentUpdaterList = []ComponentUpdater{
	ComponentUpdater{Name: "kube-apiserver", UpdateFn: DaemonsetRollingUpdate},
	ComponentUpdater{Name: "kube-scheduler", UpdateFn: DeploymentRollingUpdate},
	ComponentUpdater{Name: "kube-controller-manager", UpdateFn: DeploymentRollingUpdate},
	ComponentUpdater{Name: "kube-proxy", UpdateFn: DaemonsetRollingUpdate},
	ComponentUpdater{Name: "kubelet", UpdateFn: DaemonsetRollingUpdate},
	ComponentUpdater{Name: "on-host kubelet", UpdateFn: OnHostKubeletRollingUpdate},
}

// ClusterUpdater is responsible for safely updating an entire cluster.
type ClusterUpdater struct {
	// Client is a generic API server client.
	Client clientset.Interface
	// Components is a slice of components to be updated.
	// Each will be updated in the order they appear in the
	// slice.
	Components []ComponentUpdater
	// NewVersion is the version to upgrade to.
	NewVersion *ContainerImage
	// OldVersion is the version we are upgrading from.
	OldVersion *ContainerImage
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
type ComponentUpdater struct {
	// Name is the name of the component to update.
	Name string
	// UpdateFn is the function used to update this component.
	UpdateFn ComponentUpdaterFunc
}

// ContainerImage describes a container image. It holds
// the repo / tag for the image.
type ContainerImage struct {
	// Repo is the repository this image is in.
	Repo string
	// Tag is the image tag.
	Tag string
}

// String returns a stringified version of the Containerimage
// in the format of $repo:$tag.
func (ci *ContainerImage) String() string {
	return fmt.Sprintf("%s:%s", ci.Repo, ci.Tag)
}

// UpdaterFunc is a function that can be used to update a
// single component of a certain type. A concrete updater
// function may be something like 'DaemonSetUpdaterFunc`.
type ComponentUpdaterFunc func(clientset.Interface, string, *ContainerImage) error

// NewClusterUpdater returns a ClusterUpdater struct with defaults.
func NewClusterUpdater(client clientset.Interface, newVersion, oldVersion *ContainerImage, config *v1.ConfigMap) (*ClusterUpdater, error) {
	b := record.NewBroadcaster()
	r := b.NewRecorder(api.EventSource{
		Component: "cluster-updater",
	})
	p, err := client.Core().Pods(api.NamespaceSystem).Get(os.Getenv("MY_POD_NAME"))
	if err != nil {
		return nil, fmt.Errorf("error getting self pod: %v", err)
	}
	return &ClusterUpdater{
		Client:     client,
		Components: DefaultComponentUpdaterList,
		NewVersion: newVersion,
		OldVersion: oldVersion,
		Events:     r,
		SelfObj:    p,
	}, nil
}

// DaemonsetRollingUpdate will perform a safe rolling update on the DaemonSet
// specified by `name`. An error will be returned if the rolling update failed.
func DaemonsetRollingUpdate(client clientset.Interface, name string, image *ContainerImage) error {
	ds, err := client.Extensions().DaemonSets(api.NamespaceSystem).Get(name)
	if err != nil {
		return err
	}
	// Create new DS.
	ds.Labels["version"] = image.Tag
	ds.Spec.Template.Labels["version"] = image.Tag
	for i, c := range ds.Spec.Template.Spec.Containers {
		if c.Name == name {
			glog.Infof("updating image for container: %s", c.Name)
			ds.Spec.Template.Spec.Containers[i].Image = image.String()
			break
		}
	}
	ds, err = client.Extensions().DaemonSets(api.NamespaceSystem).Update(ds)
	if err != nil {
		return err
	}
	l, err := labels.Parse(fmt.Sprintf("k8s-app=%s", name))
	if err != nil {
		return err
	}
	lo := api.ListOptions{
		LabelSelector: l,
	}
	pl, err := client.Core().Pods(api.NamespaceSystem).List(lo)
	if err != nil {
		return err
	}
	for _, p := range pl.Items {
		// Delete old DS Pod.
		glog.Infof("Deleting pod %s", p.Name)
		err = client.Core().Pods(api.NamespaceSystem).Delete(p.Name, nil)
		if err != nil && err != io.EOF {
			return err
		}
		glog.Infof("Deleted pod %s", p.Name)
		// Wait for new DS Pod.
		for {
			glog.Info("Waiting for new pod to come up...")
			upl, err := client.Core().Pods(api.NamespaceSystem).List(lo)
			if err != nil {
				// Do not return an error here, since we have just deleted a pod,
				// it is quite possible we have deleted an API Server pod. We may fail
				// to contact it for a bit until the checkpointer API Server takes over.
				glog.Error(err)
			} else {
				if ds.Status.DesiredNumberScheduled == int32(len(upl.Items)) {
					if allContainersRunning(upl) {
						if name == "kube-apiserver" {
							// Make sure the apiserver is back online before we continue.
							re := regexp.MustCompile("v[0-9].[0-9].[0-9]")
							for {
								glog.Info("waiting until correct API Server is running")
								ver, err := client.Discovery().ServerVersion()
								if err == nil {
									cv := re.FindString(ver.GitVersion)
									ev := re.FindString(image.Tag)
									if cv != "" && cv == ev {
										break
									}
									glog.Infof("got version %s, wanted %s", cv, ev)
								}
								time.Sleep(time.Second)
							}
						}
						glog.Info("Done waiting for new pod")
						break
					}
				}
			}
			time.Sleep(time.Second)
		}
	}
	return nil
}

// DeploymentRollingUpdate will perform a safe rolling update on the Deployment
// specified by `name`. An error will be returned if the rolling update failed.
func DeploymentRollingUpdate(client clientset.Interface, name string, image *ContainerImage) error {
	// Get the current deployment.
	dep, err := client.Extensions().Deployments(api.NamespaceSystem).Get(name)
	if err != nil {
		return err
	}
	// Update the image in the specific container we care about (should match name
	// of the deployment itself, per convention).
	dep.Labels["version"] = image.Tag
	dep.Spec.Template.Labels["version"] = image.Tag
	for i, c := range dep.Spec.Template.Spec.Containers {
		if c.Name == name {
			dep.Spec.Template.Spec.Containers[i].Image = image.String()
			break
		}
	}
	// Update the deployment, which will trigger an update.
	_, err = client.Extensions().Deployments(api.NamespaceSystem).Update(dep)
	if err != nil {
		return err
	}
	// Wait for update to finish.
	l, err := labels.Parse(fmt.Sprintf("version=%s,k8s-app=%s", image.Tag, name))
	if err != nil {
		return err
	}
	lo := api.ListOptions{
		LabelSelector: l,
	}
	replicas := int32(1)
	if dep.Spec.Replicas != nil {
		replicas = *dep.Spec.Replicas
	}
	for {
		glog.Info("checking that all new containers are up")
		upl, err := client.Core().Pods(api.NamespaceSystem).List(lo)
		if err != nil {
			// TODO might be worth returning error here.
			glog.Error(err)
		} else {
			if int32(len(upl.Items)) == replicas {
				if allContainersRunning(upl) {
					return nil
				}
			}
			glog.Infof("not all new containers have started, expected %d got %d", dep.Status.Replicas, len(upl.Items))
		}
		time.Sleep(time.Second)
	}
}

func allContainersRunning(pl *v1.PodList) bool {
	for _, p := range pl.Items {
		for _, cs := range p.Status.ContainerStatuses {
			if cs.State.Running == nil {
				return false
			}
		}
	}
	return true
}

// OnHostKubeletRollingUpdate will perform a safe rolling update of the on-host kubelets
// for every node. It does this by placing annotations on each node, which the node-agent
// should react to, and then waits for the "current-config" annotation to be updated for
// each node before moving on.
func OnHostKubeletRollingUpdate(client clientset.Interface, _ string, image *ContainerImage) error {
	// foreach(node in nodes)
	// -- updateAnnotation(node)
	// -- waitForUpdatedCurrentAnnotation(node, timeout)
	var lo api.ListOptions
	nl, err := client.Core().Nodes().List(lo)
	if err != nil {
		return err
	}
	for _, n := range nl.Items {
		if n.Annotations == nil {
			n.Annotations = make(map[string]string)
		}
		n.Annotations[node.DesiredVersionAnnotation] = image.String()
		_, err = client.Core().Nodes().Update(&n)
		if err != nil {
			return err
		}
		lo := api.ListOptions{
			Watch:         true,
			FieldSelector: fields.OneTermEqualSelector("metadata.name", n.Name),
		}
		w, err := client.Core().Nodes().Watch(lo)
		if err != nil {
			return err
		}
		for e := range w.ResultChan() {
			switch e.Type {
			case watch.Modified:
				nn, ok := e.Object.(*v1.Node)
				if !ok {
					glog.Infof("recieved unexpected type from Node watch. Expected *v1.Node, got: %T", e.Object.GetObjectKind())
					continue
				}
				if nn.Annotations[node.DesiredVersionAnnotation] == nn.Annotations[node.CurrentVersionAnnotation] {
					break
				}
				glog.Infof("Node %s update completed", nn.Name)
				break
			case watch.Error:
				aerr, ok := e.Object.(*unversioned.Status)
				if !ok {
					glog.Infof("recieved unexpected error type from Node watch. Expected *api.Status, got: %T", e.Object.GetObjectKind())
					continue
				}
				glog.Errorf("recieved error during watch on Node %s: %s", n.Name, aerr.Message)
			default:
				glog.Infof("recieved event %s while watching node update, ignoring", e.Type)
			}
		}
	}
	return nil
}

// Update will update every component in the list of known components for
// the instance of the ClusterUpdater.
func (cu *ClusterUpdater) Update() error {
	for _, c := range cu.Components {
		cu.Events.Eventf(cu.SelfObj, api.EventTypeNormal, "Begin update of component: %s", c.Name)
		glog.Infof("Begin update of component: %s", c.Name)
		if err := c.UpdateFn(cu.Client, c.Name, cu.NewVersion); err != nil {
			cu.Events.Eventf(cu.SelfObj, api.EventTypeWarning,
				"Failed update of component: %s, rolling back to version: %s from %s",
				c.Name, cu.OldVersion, cu.NewVersion)
			glog.Errorf("Failed update of component: %s, rolling back to version: %s from %s due to: %v",
				c.Name, cu.OldVersion, cu.NewVersion, err)
			return err
		}
		glog.Infof("Finshed update of componenet: %s", c.Name)
		cu.Events.Eventf(cu.SelfObj, api.EventTypeNormal, "Finished update of component: %s", c.Name)
	}
	return nil
}
