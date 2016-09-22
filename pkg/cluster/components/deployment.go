package components

import (
	"fmt"
	"strconv"
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/release_1_3"
	"k8s.io/kubernetes/pkg/util/deployment"
)

// DeploymentUpdater is responsible for updating a Deployment.
type DeploymentUpdater struct {
	// name of the Deployment object.
	name string
	// client is an API Server client.
	client clientset.Interface
	// priority is the priority to update this Deployment.
	priority int
}

func NewDeploymentUpdater(client clientset.Interface, dep *extensions.Deployment) (*DeploymentUpdater, error) {
	if dep.Annotations == nil {
		return nil, noAnnotationError("Deployment", dep.Name)
	}
	ps, ok := dep.Annotations[updatePriorityAnnotation]
	if !ok {
		return nil, noAnnotationError("Deployment", dep.Name)
	}
	priority, err := strconv.Atoi(ps)
	if err != nil {
		return nil, err
	}
	return &DeploymentUpdater{
		name:     dep.Name,
		client:   client,
		priority: priority,
	}, nil
}

// Name returns the name of the Deployment this updater
// is responsible for.
func (du *DeploymentUpdater) Name() string {
	return du.name
}

// Version returns the version of the Deployment.
func (du *DeploymentUpdater) Version() (*Version, error) {
	dep, err := du.client.Extensions().Deployments(api.NamespaceSystem).Get(du.Name())
	if err != nil {
		return nil, err
	}
	for i, c := range dep.Spec.Template.Spec.Containers {
		if c.Name == du.Name() {
			return ParseVersionFromImage(dep.Spec.Template.Spec.Containers[i].Image)
		}
	}
	return nil, fmt.Errorf("could not determine version for Deployment %s", du.Name())
}

// Priority is the priority to update this Deployment.
func (du *DeploymentUpdater) Priority() int {
	return du.priority
}

// UpdateToVersion will update the Deployment to the given version.
func (du *DeploymentUpdater) UpdateToVersion(v *Version) (bool, error) {
	// Get the current deployment.
	dep, err := du.client.Extensions().Deployments(api.NamespaceSystem).Get(du.Name())
	if err != nil {
		return false, err
	}
	// Update the image in the specific container we care about (should match name
	// of the deployment itself, per convention).
	for i, c := range dep.Spec.Template.Spec.Containers {
		if c.Name == du.Name() {
			dv, err := ParseVersionFromImage(dep.Spec.Template.Spec.Containers[i].Image)
			if err != nil {
				return false, err
			}
			// TODO we should also check that an update is not
			// already in progress. It's possible we bailed
			// before it was finished, and when we retry we
			// should wait until it's finished if it is in progress.
			if dv.Semver().EQ(v.Semver()) {
				return false, nil
			}
			dep.Spec.Template.Spec.Containers[i].Image = v.image.String()
			break
		}
	}
	oldGeneration := dep.Status.ObservedGeneration
	// Update the deployment, which will trigger an update.
	_, err = du.client.Extensions().Deployments(api.NamespaceSystem).Update(dep)
	if err != nil {
		return false, err
	}

	err = deployment.WaitForObservedDeployment(func() (*extensions.Deployment, error) {
		dp, err := du.client.Extensions().Deployments(api.NamespaceSystem).Get(du.Name())
		if err != nil {
			return nil, err
		}
		var out extensions.Deployment
		v1beta1.Convert_v1beta1_Deployment_To_extensions_Deployment(dp, &out, nil)
		return &out, nil
	}, oldGeneration+1, time.Second, 10*time.Minute)
	if err != nil {
		return false, err
	}
	return true, nil
}
