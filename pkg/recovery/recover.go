// Package recovery provides tooling to help with control plane disaster recovery. Recover() uses a
// Backend to extract the control plane from a store, such as etcd, and use those to write assets
// that can be used by `bootkube start` to reboot the control plane.
//
// The recovery tool assumes that the component names for the control plane elements are the same as
// what is output by `bootkube render`. The `bootkube start` command also makes this assumption.
// It also assumes that kubeconfig on the kubelet is located at /etc/kubernetes/kubeconfig, though
// that can be changed in the bootstrap manifests that are rendered.
package recovery

import (
	"context"
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/ghodss/yaml"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"

	"github.com/kubernetes-incubator/bootkube/pkg/asset"
)

const (
	k8sAppLabel            = "k8s-app"   // The label used in versions > v0.4.2
	componentAppLabel      = "component" // The label used in versions <= v0.4.2
	kubeletKubeConfigPath  = "/etc/kubernetes"
	apiServerContainerName = "kube-apiserver"
)

var (
	// bootstrapK8sApps contains the components (as identified by the the label "k8s-app") that we
	// will extract to construct the temporary bootstrap control plane.
	bootstrapK8sApps = map[string]struct{}{
		apiServerContainerName:    {},
		"kube-controller-manager": {},
		"kube-scheduler":          {},
	}
	// kubeConfigK8sContainers contains the names of the bootstrap container specs that need to add a
	// --kubeconfig flag to run in non-self-hosted mode.
	kubeConfigK8sContainers = map[string]struct{}{
		"kube-controller-manager": {},
		"kube-scheduler":          {},
	}
	// typeMetas contains a mapping from API object types to the TypeMeta struct that should be
	// populated for them when they are serialized.
	typeMetas    = make(map[reflect.Type]metav1.TypeMeta)
	metaAccessor = meta.NewAccessor()
)

func init() {
	addTypeMeta := func(obj runtime.Object, gv schema.GroupVersion) {
		t := reflect.TypeOf(obj)
		typeMetas[t] = metav1.TypeMeta{
			APIVersion: gv.String(),
			Kind:       t.Elem().Name(),
		}
	}
	addTypeMeta(&v1.ConfigMap{}, v1.SchemeGroupVersion)
	addTypeMeta(&v1beta1.DaemonSet{}, v1beta1.SchemeGroupVersion)
	addTypeMeta(&v1beta1.Deployment{}, v1beta1.SchemeGroupVersion)
	addTypeMeta(&v1.Pod{}, v1.SchemeGroupVersion)
	addTypeMeta(&v1.Secret{}, v1.SchemeGroupVersion)
}

// Backend defines an interface for any backend that can populate a controlPlane struct.
type Backend interface {
	read(context.Context) (*controlPlane, error)
}

// controlPlane holds the control plane objects that are recovered from a backend.
type controlPlane struct {
	configMaps  v1.ConfigMapList
	daemonSets  v1beta1.DaemonSetList
	deployments v1beta1.DeploymentList
	secrets     v1.SecretList

	// for self hosted etcd recovery if not nil
	bootEtcd        *asset.Asset
	bootEtcdService *asset.Asset
	tpr             *asset.Asset
}

// Recover recovers a control plane using the provided backend and kubeConfigPath, returning assets
// for the existing control plane and a bootstrap control plane that can be used with `bootkube
// start` to re-bootstrap the control plane.
func Recover(ctx context.Context, backend Backend, kubeConfigPath string) (asset.Assets, error) {
	cp, err := backend.read(ctx)
	if err != nil {
		return nil, err
	}

	as, err := cp.renderBootstrap()
	if err != nil {
		return nil, err
	}

	kc, err := renderKubeConfig(kubeConfigPath)
	if err != nil {
		return nil, err
	}
	as = append(as, kc)

	return as, nil
}

// renderBootstrap returns assets for a bootstrap control plane that can be used with `bootkube
// start` to re-bootstrap a control plane. These assets are derived from the self-hosted control
// plane that was recovered by the backend, but modified for direct injection into a kubelet.
func (cp *controlPlane) renderBootstrap() (asset.Assets, error) {
	pods, err := extractBootstrapPods(cp.daemonSets.Items, cp.deployments.Items)
	if err != nil {
		return nil, err
	}
	isSelfHostedEtcd := cp.bootEtcd != nil
	requiredConfigMaps, requiredSecrets := fixUpBootstrapPods(pods, isSelfHostedEtcd)
	as, err := outputBootstrapPods(pods)
	if err != nil {
		return nil, err
	}
	configMaps, err := outputBootstrapConfigMaps(cp.configMaps, requiredConfigMaps)
	if err != nil {
		return nil, err
	}
	as = append(as, configMaps...)

	if isSelfHostedEtcd {
		requiredSecrets[asset.SecretEtcdMemberPeer] = filepath.Dir(asset.AssetPathEtcdCA)
		requiredSecrets[asset.SecretEtcdMemberCli] = filepath.Dir(asset.AssetPathEtcdCA)
		requiredSecrets[asset.SecretEtcdOperator] = filepath.Dir(asset.AssetPathEtcdCA)
	}
	secrets, err := outputBootstrapSecrets(cp.secrets, requiredSecrets)
	if err != nil {
		return nil, err
	}
	as = append(as, secrets...)
	if cp.bootEtcd != nil {
		as = append(as, *cp.bootEtcd)
		as = append(as, *cp.bootEtcdService)
		as = append(as, *cp.tpr)
	}
	return as, nil
}

// extractBootstrapPods extracts bootstrap pod specs from daemonsets and deployments.
func extractBootstrapPods(daemonSets []v1beta1.DaemonSet, deployments []v1beta1.Deployment) ([]v1.Pod, error) {
	var pods []v1.Pod
	for _, ds := range daemonSets {
		if isBootstrapApp(ds.Labels) {
			pod := v1.Pod{Spec: ds.Spec.Template.Spec}
			if err := setBootstrapPodMetadata(&pod, ds.ObjectMeta); err != nil {
				return nil, err
			}
			pods = append(pods, pod)
		}
	}
	for _, ds := range deployments {
		if isBootstrapApp(ds.Labels) {
			pod := v1.Pod{Spec: ds.Spec.Template.Spec}
			if err := setBootstrapPodMetadata(&pod, ds.ObjectMeta); err != nil {
				return nil, err
			}
			pods = append(pods, pod)
		}
	}
	return pods, nil
}

// isBootstrapApp returns true if this app belongs to the bootstrap control plane, based on its
// labels.
func isBootstrapApp(labels map[string]string) bool {
	k8sApp := labels[k8sAppLabel]
	if k8sApp == "" {
		k8sApp = labels[componentAppLabel]
	}
	_, ok := bootstrapK8sApps[k8sApp]
	return ok
}

// setBootstrapPodMetadata creates valid metadata for a bootstrap pod. Currently it sets the
// TypeMeta and Name, Namespace, and Annotations on the ObjectMeta.
func setBootstrapPodMetadata(pod *v1.Pod, parent metav1.ObjectMeta) error {
	if err := setTypeMeta(pod); err != nil {
		return err
	}
	pod.ObjectMeta = metav1.ObjectMeta{
		Annotations: parent.Annotations,
		Name:        "bootstrap-" + parent.Name,
		Namespace:   parent.Namespace,
	}
	return nil
}

// fixUpBootstrapPods modifies extracted bootstrap pod specs to have correct metadata and point to
// filesystem-mount-based secrets, and removes any security contexts that might prevent the pods
// from accessing those secrets. It returns mappings from configMap and secret names to output
// paths that must also be rendered in order for the bootstrap pods to be functional.
// If selfHostedEtcd is true, it also fixes up the etcd servers flag for the API server.
func fixUpBootstrapPods(pods []v1.Pod, selfHostedEtcd bool) (requiredConfigMaps, requiredSecrets map[string]string) {
	requiredConfigMaps, requiredSecrets = make(map[string]string), make(map[string]string)
	for i := range pods {
		pod := &pods[i]

		// Fix SecurityContext to ensure the pod runs as root.
		if pod.Spec.SecurityContext != nil {
			pod.Spec.SecurityContext.RunAsNonRoot = nil
			pod.Spec.SecurityContext.RunAsUser = nil
		}

		// Change secret volumes to point to file mounts.
		for i := range pod.Spec.Volumes {
			vol := &pod.Spec.Volumes[i]
			if vol.Secret != nil {
				pathSuffix := filepath.Join("secrets", vol.Secret.SecretName)
				requiredSecrets[vol.Secret.SecretName] = filepath.Join(asset.AssetPathSecrets, pathSuffix)
				vol.HostPath = &v1.HostPathVolumeSource{Path: filepath.Join(asset.BootstrapSecretsDir, pathSuffix)}
				vol.Secret = nil
			} else if vol.ConfigMap != nil {
				pathSuffix := filepath.Join("config-maps", vol.ConfigMap.Name)
				requiredConfigMaps[vol.ConfigMap.Name] = filepath.Join(asset.AssetPathSecrets, pathSuffix)
				vol.HostPath = &v1.HostPathVolumeSource{Path: path.Join(asset.BootstrapSecretsDir, pathSuffix)}
				vol.ConfigMap = nil
			}
		}

		// Make sure the kubeconfig is in the commandline.
		for i := range pod.Spec.Containers {
			cn := &pod.Spec.Containers[i]

			// Fix SecurityContext to ensure the container runs as root.
			if cn.SecurityContext != nil {
				cn.SecurityContext.RunAsNonRoot = nil
				cn.SecurityContext.RunAsUser = nil
			}

			// Assumes the bootkube naming convention is used. Could also just make sure the image uses hyperkube.
			if _, ok := kubeConfigK8sContainers[cn.Name]; ok {
				cn.Command = append(cn.Command, "--kubeconfig=/kubeconfig/kubeconfig")
				cn.VolumeMounts = append(cn.VolumeMounts, v1.VolumeMount{
					MountPath: "/kubeconfig",
					Name:      "kubeconfig",
					ReadOnly:  true,
				})
			}

			if selfHostedEtcd && cn.Name == apiServerContainerName {
				for i, cm := range cn.Command {
					if strings.Contains(cm, "--etcd-servers") {
						cn.Command[i] = cm + ",http://127.0.0.1:12379"
					}
				}
			}
		}

		// Add a mount for the kubeconfig.
		pod.Spec.Volumes = append(pod.Spec.Volumes, v1.Volume{
			VolumeSource: v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: kubeletKubeConfigPath}},
			Name:         "kubeconfig",
		})
	}
	return
}

// outputBootstrapPods outputs the bootstrap pod definitions.
func outputBootstrapPods(pods []v1.Pod) (asset.Assets, error) {
	var as asset.Assets
	for _, pod := range pods {
		a, err := serializeObjToYAML(path.Join(asset.AssetPathBootstrapManifests, pod.Name+".yaml"), &pod)
		if err != nil {
			return nil, err
		}
		as = append(as, a)
	}
	return as, nil
}

// outputBootstrapConfigMaps creates assets for all the configMap names in the requiredConfigMaps
// set. It returns an error if any configMap cannot be found in the provided configMaps list.
func outputBootstrapConfigMaps(configMaps v1.ConfigMapList, requiredConfigMaps map[string]string) (asset.Assets, error) {
	return outputKeyValueData(&configMaps, requiredConfigMaps, func(obj runtime.Object) map[string][]byte {
		configMap, ok := obj.(*v1.ConfigMap)
		if !ok || configMap == nil {
			return nil
		}
		output := make(map[string][]byte)
		for k, v := range configMap.Data {
			output[k] = []byte(v)
		}
		return output
	})
}

// outputBootstrapSecrets creates assets for all the secret names in the requiredSecrets set. It
// returns an error if any secret cannot be found in the provided secrets list.
func outputBootstrapSecrets(secrets v1.SecretList, requiredSecrets map[string]string) (asset.Assets, error) {
	return outputKeyValueData(&secrets, requiredSecrets, func(obj runtime.Object) map[string][]byte {
		if secret, ok := obj.(*v1.Secret); ok && secret != nil {
			return secret.Data
		}
		return nil
	})
}

// outputKeyValueData takes a key-value object (such as a Secret or ConfigMap) and outputs assets
// for each key-value pair. See outputBootstrapConfigMaps or outputBootstrapSecrets for usage.
func outputKeyValueData(objList runtime.Object, requiredObjs map[string]string, extractData func(runtime.Object) map[string][]byte) (asset.Assets, error) {
	var as asset.Assets
	objs, err := meta.ExtractList(objList)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		name, err := metaAccessor.Name(obj)
		if err != nil {
			return nil, err
		}
		if namePrefix, ok := requiredObjs[name]; ok {
			for key, data := range extractData(obj) {
				as = append(as, asset.Asset{
					Name: path.Join(namePrefix, key),
					Data: data,
				})
			}
			delete(requiredObjs, name)
		}
	}
	if len(requiredObjs) > 0 {
		var missingObjs []string
		for obj := range requiredObjs {
			missingObjs = append(missingObjs, obj)
		}
		return nil, fmt.Errorf("failed to extract some required objects: %v", missingObjs)
	}
	return as, nil
}

// renderKubeConfig outputs kubeconfig assets to ensure that the kubeconfig will be rendered to the
// assetDir for use by `bootkube start`.
func renderKubeConfig(kubeConfigPath string) (asset.Asset, error) {
	kubeConfig, err := ioutil.ReadFile(kubeConfigPath)
	if err != nil {
		return asset.Asset{}, err
	}
	return asset.Asset{
		Name: asset.AssetPathKubeConfig, // used by `bootkube start`.
		Data: kubeConfig,
	}, nil
}

// setTypeMeta sets the TypeMeta for a runtime.Object.
// TODO(diegs): find the apimachinery code that does this, and use that instead.
func setTypeMeta(obj runtime.Object) error {
	typeMeta, ok := typeMetas[reflect.TypeOf(obj)]
	if !ok {
		return fmt.Errorf("don't know about type: %T", obj)
	}
	metaAccessor.SetAPIVersion(obj, typeMeta.APIVersion)
	metaAccessor.SetKind(obj, typeMeta.Kind)
	return nil
}

// serializeObjToYAML serializes a runtime.Object into a YAML asset.
func serializeObjToYAML(assetName string, obj runtime.Object) (asset.Asset, error) {
	data, err := yaml.Marshal(obj)
	if err != nil {
		return asset.Asset{}, err
	}
	return asset.Asset{
		Name: assetName,
		Data: data,
	}, nil
}
