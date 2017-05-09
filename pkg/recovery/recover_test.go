package recovery

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"

	"github.com/kubernetes-incubator/bootkube/pkg/asset"
)

var (
	secretData = []byte("this is very secret")

	cp = &controlPlane{
		configMaps: v1.ConfigMapList{},
		daemonSets: v1beta1.DaemonSetList{
			Items: []v1beta1.DaemonSet{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kube-apiserver",
					Namespace: "kube-system",
					Labels: map[string]string{
						"tier":    "control-plane",
						"k8s-app": "kube-apiserver",
					},
				},
				Spec: v1beta1.DaemonSetSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Containers: []v1.Container{{
								Name:    "kube-apiserver",
								Image:   "quay.io/coreos/hyperkube:v1.6.2_coreos.0",
								Command: []string{"/usr/bin/flock", "/hyperkube", "apiserver", "--secure-port=443"},
								VolumeMounts: []v1.VolumeMount{{
									Name:      "ssl-certs-host",
									MountPath: "/etc/ssl/certs",
									ReadOnly:  true,
								}, {
									Name:      "secrets",
									MountPath: "/etc/kubernetes/secrets",
									ReadOnly:  true,
								}},
							}},
							Volumes: []v1.Volume{{
								Name:         "ssl-certs-host",
								VolumeSource: v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: "/usr/share/ca-certificates"}},
							}, {
								Name:         "secrets",
								VolumeSource: v1.VolumeSource{Secret: &v1.SecretVolumeSource{SecretName: "kube-apiserver"}},
							}},
						},
					},
				},
			}},
		},
		deployments: v1beta1.DeploymentList{
			Items: []v1beta1.Deployment{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kube-scheduler",
					Namespace: "kube-system",
					Labels: map[string]string{
						"tier":    "control-plane",
						"k8s-app": "kube-scheduler",
					},
				},
				Spec: v1beta1.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Containers: []v1.Container{{
								Name:    "kube-scheduler",
								Image:   "quay.io/coreos/hyperkube:v1.6.2_coreos.0",
								Command: []string{"/hyperkube", "scheduler"},
							}},
						},
					},
				},
			}},
		},
		secrets: v1.SecretList{
			Items: []v1.Secret{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kube-apiserver",
					Namespace: "kube-system",
				},
				Data: map[string][]byte{"apiserver.crt": secretData},
			}},
		},
	}
)

func TestExtractBootstrapPods(t *testing.T) {
	got, err := extractBootstrapPods(cp.daemonSets.Items, cp.deployments.Items)
	want := []v1.Pod{{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bootstrap-kube-apiserver",
			Namespace: "kube-system",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{
				Name:    "kube-apiserver",
				Image:   "quay.io/coreos/hyperkube:v1.6.2_coreos.0",
				Command: []string{"/usr/bin/flock", "/hyperkube", "apiserver", "--secure-port=443"},
				VolumeMounts: []v1.VolumeMount{{
					Name:      "ssl-certs-host",
					MountPath: "/etc/ssl/certs",
					ReadOnly:  true,
				}, {
					Name:      "secrets",
					MountPath: "/etc/kubernetes/secrets",
					ReadOnly:  true,
				}},
			}},
			Volumes: []v1.Volume{{
				Name:         "ssl-certs-host",
				VolumeSource: v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: "/usr/share/ca-certificates"}},
			}, {
				Name:         "secrets",
				VolumeSource: v1.VolumeSource{Secret: &v1.SecretVolumeSource{SecretName: "kube-apiserver"}},
			}},
		},
	}, {
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bootstrap-kube-scheduler",
			Namespace: "kube-system",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{
				Name:    "kube-scheduler",
				Image:   "quay.io/coreos/hyperkube:v1.6.2_coreos.0",
				Command: []string{"/hyperkube", "scheduler"},
			}},
		},
	}}
	if err != nil {
		t.Errorf("extractBootstrapPods(%v, %v) = %v, want: %v", cp.daemonSets.Items, cp.deployments.Items, err, nil)
	} else if !reflect.DeepEqual(got, want) {
		t.Errorf("extractBootstrapPods(%v, %v) = %v, want: %v", cp.daemonSets.Items, cp.deployments.Items, got, want)
	}
}

func TestFixUpBootstrapPods(t *testing.T) {
	pods := []v1.Pod{{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bootstrap-kube-apiserver",
			Namespace: "kube-system",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{
				Name:    "kube-apiserver",
				Image:   "quay.io/coreos/hyperkube:v1.6.2_coreos.0",
				Command: []string{"/usr/bin/flock", "/hyperkube", "apiserver", "--secure-port=443"},
				VolumeMounts: []v1.VolumeMount{{
					Name:      "ssl-certs-host",
					MountPath: "/etc/ssl/certs",
					ReadOnly:  true,
				}, {
					Name:      "secrets",
					MountPath: "/etc/kubernetes/secrets",
					ReadOnly:  true,
				}},
			}},
			Volumes: []v1.Volume{{
				Name:         "ssl-certs-host",
				VolumeSource: v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: "/usr/share/ca-certificates"}},
			}, {
				Name:         "secrets",
				VolumeSource: v1.VolumeSource{Secret: &v1.SecretVolumeSource{SecretName: "kube-apiserver"}},
			}},
		},
	}, {
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bootstrap-kube-scheduler",
			Namespace: "kube-system",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{
				Name:    "kube-scheduler",
				Image:   "quay.io/coreos/hyperkube:v1.6.2_coreos.0",
				Command: []string{"/hyperkube", "scheduler"},
			}},
		},
	}}
	wantPods := []v1.Pod{{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bootstrap-kube-apiserver",
			Namespace: "kube-system",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{
				Name:    "kube-apiserver",
				Image:   "quay.io/coreos/hyperkube:v1.6.2_coreos.0",
				Command: []string{"/usr/bin/flock", "/hyperkube", "apiserver", "--secure-port=443"},
				VolumeMounts: []v1.VolumeMount{{
					Name:      "ssl-certs-host",
					MountPath: "/etc/ssl/certs",
					ReadOnly:  true,
				}, {
					Name:      "secrets",
					MountPath: "/etc/kubernetes/secrets",
					ReadOnly:  true,
				}},
			}},
			Volumes: []v1.Volume{{
				Name:         "ssl-certs-host",
				VolumeSource: v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: "/usr/share/ca-certificates"}},
			}, {
				Name:         "secrets",
				VolumeSource: v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: "/etc/kubernetes/bootstrap-secrets/kube-apiserver"}},
			}, {
				Name:         "kubeconfig",
				VolumeSource: v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: "/etc/kubernetes/kubeconfig"}},
			}},
		},
	}, {
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bootstrap-kube-scheduler",
			Namespace: "kube-system",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{
				Name:    "kube-scheduler",
				Image:   "quay.io/coreos/hyperkube:v1.6.2_coreos.0",
				Command: []string{"/hyperkube", "scheduler", "--kubeconfig=/kubeconfig/kubeconfig"},
				VolumeMounts: []v1.VolumeMount{{
					Name:      "kubeconfig",
					MountPath: "/kubeconfig",
					ReadOnly:  true,
				}},
			}},
			Volumes: []v1.Volume{{
				Name:         "kubeconfig",
				VolumeSource: v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: "/etc/kubernetes/kubeconfig"}},
			}},
		},
	}}
	wantSecrets := map[string]struct{}{"kube-apiserver": {}}
	gotSecrets, err := fixUpBootstrapPods(pods)
	if err != nil || !reflect.DeepEqual(gotSecrets, wantSecrets) {
		t.Errorf("fixUpBootstrapPods(%v) = %v, %v, want: %v, %v", pods, gotSecrets, err, wantSecrets, nil)
	} else if !reflect.DeepEqual(pods, wantPods) {
		t.Errorf("fixUpBootstrapPods(%v) = %v, want: %v", pods, pods, wantPods)
	}
}

func TestOutputBootstrapSecrets(t *testing.T) {
	requiredSecrets := map[string]struct{}{"kube-apiserver": {}}
	want := asset.Assets{{
		Name: "tls/kube-apiserver/apiserver.crt",
		Data: secretData,
	}}
	if got, err := outputBootstrapSecrets(cp.secrets.Items, requiredSecrets); err != nil {
		t.Errorf("outputBootstrapSecrets(%v, %v) = %v, want: nil", cp.secrets.Items, requiredSecrets, err)
	} else if !reflect.DeepEqual(got, want) {
		t.Errorf("outputBootstrapSecrets(%v, %v) = %v, want: %v", cp.secrets.Items, requiredSecrets, got, want)
	}
}

func TestOutputBootstrapSecretsMissing(t *testing.T) {
	requiredSecrets := map[string]struct{}{"missing-secret": {}}
	if as, err := outputBootstrapSecrets(cp.secrets.Items, requiredSecrets); err == nil {
		t.Errorf("outputBootstrapSecrets(%v, %v) = %v, %v, want: nil, non-nil", cp.secrets.Items, requiredSecrets, as, err)
	}
}

func TestIsBootstrapApp(t *testing.T) {
	for app := range bootstrapK8sApps {
		labels := map[string]string{
			"tier":      "control-plane",
			k8sAppLabel: app,
		}
		if !isBootstrapApp(labels) {
			t.Errorf("isBootstrapApp(%v) = false, want: true", labels)
		}
		labels = map[string]string{
			"tier":            "control-plane",
			componentAppLabel: app,
		}
		if !isBootstrapApp(labels) {
			t.Errorf("isBootstrapApp(%v) = false, want: true", labels)
		}
	}
}

func TestIsNotBootstrapApp(t *testing.T) {
	for _, labels := range []map[string]string{{
		"tier":      "control-plane",
		k8sAppLabel: "wrong-app",
	}, {
		"tier":        "control-plane",
		"wrong-label": "kube-apiserver",
	}} {
		if isBootstrapApp(labels) {
			t.Errorf("isBootstrapApp(%v) = true, want: false", labels)
		}
	}
}

func TestSetTypeMeta(t *testing.T) {
	for _, obj := range []runtime.Object{
		&v1.ConfigMap{},
		&v1beta1.DaemonSet{},
		&v1beta1.Deployment{},
		&v1.Pod{},
		&v1.Secret{},
	} {
		if err := setTypeMeta(obj); err != nil {
			t.Errorf("setTypeMeta(%v) = %v, want: nil", obj, err)
		}
		if apiVersion, err := metaAccessor.APIVersion(obj); apiVersion == "" || err != nil {
			t.Errorf("APIVersion(%v) = %v, %v, want: <non-empty>, nil", obj, apiVersion, err)
		}
		if kind, err := metaAccessor.Kind(obj); kind == "" || err != nil {
			t.Errorf("Kind(%v) = %v, %v, want: <non-empty>, nil", obj, kind, err)
		}
	}
}
