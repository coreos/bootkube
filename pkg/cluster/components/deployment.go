package components

import (
	"fmt"
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
	// Client is an API Server client.
	client clientset.Interface
}

func NewDeploymentUpdater(client clientset.Interface, dep *v1beta1.Deployment) *DeploymentUpdater {
	return &DeploymentUpdater{
		name:   dep.Name,
		client: client,
	}
}

// Name returns the name of the Deployment this updater
// is responsible for.
func (dsu *DeploymentUpdater) Name() string {
	return dsu.name
}

// CurrentVersion is the current version of this Deployment.
func (du *DeploymentUpdater) CurrentVersion() (*Version, error) {
	dp, err := du.client.Extensions().Deployments(api.NamespaceSystem).Get(du.Name())
	if err != nil {
		return nil, err
	}
	for _, c := range dp.Spec.Template.Spec.Containers {
		if c.Name == du.Name() {
			return ParseVersionFromImage(c.Image)
		}
	}
	return nil, fmt.Errorf("unable to get current version for Deployment %s", du.Name())
}

// UpdateToVersion will update the Deployment to the given version.
func (du *DeploymentUpdater) UpdateToVersion(client clientset.Interface, v *Version) error {
	// Get the current deployment.
	dep, err := du.client.Extensions().Deployments(api.NamespaceSystem).Get(du.Name())
	if err != nil {
		return err
	}
	// Update the image in the specific container we care about (should match name
	// of the deployment itself, per convention).
	dep.Labels["version"] = v.Image.Tag
	dep.Spec.Template.Labels["version"] = v.Image.Tag
	for i, c := range dep.Spec.Template.Spec.Containers {
		if c.Name == du.Name() {
			dep.Spec.Template.Spec.Containers[i].Image = v.Image.String()
			break
		}
	}
	oldGeneration := dep.Status.ObservedGeneration
	// Update the deployment, which will trigger an update.
	_, err = du.client.Extensions().Deployments(api.NamespaceSystem).Update(dep)
	if err != nil {
		return err
	}

	return deployment.WaitForObservedDeployment(func() (*extensions.Deployment, error) {
		dp, err := du.client.Extensions().Deployments(api.NamespaceSystem).Get(du.Name())
		if err != nil {
			return nil, err
		}
		var out extensions.Deployment
		v1beta1.Convert_v1beta1_Deployment_To_extensions_Deployment(dp, &out, nil)
		return &out, nil
	}, oldGeneration+1, time.Second, 10*time.Minute)
}
