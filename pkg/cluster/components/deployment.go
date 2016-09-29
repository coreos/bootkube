package components

import (
	"fmt"
	"strconv"
	"time"

	"github.com/kubernetes-incubator/bootkube/pkg/cluster/components/version"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/util/deployment"
	"k8s.io/kubernetes/pkg/util/wait"
)

const (
	newReplicaTimeout       = 2 * time.Minute
	deploymentUpdateTimeout = 5 * time.Minute
)

// DeploymentUpdater is responsible for updating a Deployment.
type DeploymentUpdater struct {
	// name of the Deployment object.
	name string
	// client is an API Server client.
	client         unversioned.Interface
	internalclient internalclientset.Interface

	// priority is the priority to update this Deployment.
	priority int
}

func NewDeploymentUpdater(client unversioned.Interface, internalclient internalclientset.Interface, dep *extensions.Deployment) (*DeploymentUpdater, error) {
	if dep.Annotations == nil {
		return nil, version.NoAnnotationError("Deployment", dep.Name)
	}
	ps, ok := dep.Annotations[version.UpdatePriorityAnnotation]
	if !ok {
		return nil, version.NoAnnotationError("Deployment", dep.Name)
	}
	priority, err := strconv.Atoi(ps)
	if err != nil {
		return nil, err
	}
	return &DeploymentUpdater{
		name:           dep.Name,
		client:         client,
		internalclient: internalclient,
		priority:       priority,
	}, nil
}

// Name returns the name of the Deployment this updater
// is responsible for.
func (du *DeploymentUpdater) Name() string {
	return du.name
}

// Version returns the version of the Deployment.
func (du *DeploymentUpdater) Version() (*version.Version, error) {
	dep, err := du.client.Extensions().Deployments(api.NamespaceSystem).Get(du.Name())
	if err != nil {
		return nil, err
	}
	for i, c := range dep.Spec.Template.Spec.Containers {
		if c.Name == du.Name() {
			return version.ParseFromImageString(dep.Spec.Template.Spec.Containers[i].Image)
		}
	}
	return nil, fmt.Errorf("could not determine version for Deployment %s", du.Name())
}

// Priority is the priority to update this Deployment.
func (du *DeploymentUpdater) Priority() int {
	return du.priority
}

// UpdateToVersion will update the Deployment to the given version.
func (du *DeploymentUpdater) UpdateToVersion(v *version.Version) (bool, error) {
	// Get the current deployment.
	dep, err := du.client.Extensions().Deployments(api.NamespaceSystem).Get(du.Name())
	if err != nil {
		return false, err
	}
	// Update the image in the specific container we care about (should match name
	// of the deployment itself, per convention).
	var updatedDep *extensions.Deployment
	for i, c := range dep.Spec.Template.Spec.Containers {
		if c.Name == du.Name() {
			// Check if this deployment has been updated by checking the version of
			// image that is being ran.
			if dep.Spec.Template.Spec.Containers[i].Image != v.ImageString() {
				dep.Spec.Template.Spec.Containers[i].Image = v.ImageString()
				// Update the deployment, which will trigger an update.
				updatedDep, err = du.client.Extensions().Deployments(api.NamespaceSystem).Update(dep)
				if err != nil {
					return false, err
				}
			} else {
				// Since we know above that this Deployment is targeting
				// the new version, we must next check whether the Deployment
				// is still in the progress of an update. We do this by getting a
				// list of old RSes targeted by this Deployment, specifically only the ones
				// that still have Pods. If that count > 0, we're still updating.
				// Otherwise, we're all good, and we can move onto the next component.
				oldRSWithPods, _, err := deployment.GetOldReplicaSets(dep, du.internalclient)
				if err != nil {
					return false, err
				}
				if len(oldRSWithPods) == 0 {
					return false, nil
				}
			}
			break
		}
	}
	if updatedDep == nil {
		updatedDep = dep
	}
	var newRS *extensions.ReplicaSet
	err = wait.Poll(time.Second, newReplicaTimeout, func() (bool, error) {
		newRS, err = deployment.GetNewReplicaSet(updatedDep, du.internalclient)
		if err != nil {
			return false, err
		}
		return newRS != nil, nil
	})
	if err != nil {
		return false, err
	}
	err = wait.Poll(time.Second, deploymentUpdateTimeout, func() (bool, error) {
		count, err := deployment.GetAvailablePodsForReplicaSets(du.internalclient, updatedDep, []*extensions.ReplicaSet{newRS}, 5)
		if err != nil {
			return false, err
		}
		return count == newRS.Spec.Replicas, nil
	})
	if err != nil {
		return false, err
	}
	return true, nil
}
