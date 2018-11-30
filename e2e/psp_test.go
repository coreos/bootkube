package e2e

import (
	"fmt"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	"k8s.io/api/rbac/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func newPrivilegedPod(namespace string) *v1.Pod {
	privileged := true
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "privileged-pod",
			Namespace: namespace,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "nginx",
					Image: "nginx",
					SecurityContext: &v1.SecurityContext{
						Privileged: &privileged,
					},
				},
			},
		},
	}
}

func newDeployment(name, namespace, image string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"run": name,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"run": name,
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"run": name,
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						v1.Container{
							Name:  name,
							Image: image,
						},
					},
				},
			},
		},
	}
}

func TestPSPCreatePrivilegedPod(t *testing.T) {
	adminConfig := newRestConfig(t)
	adminClient := kubernetes.NewForConfigOrDie(adminConfig)

	// controllerClient is a client that acts as the "replicatset-controller"
	// through user impersonation.
	//
	// We want to make sure this controller doesn't normally have the ability to
	// create privileged pods in new namespaces.
	//
	// https://kubernetes.io/docs/admin/authentication/#user-impersonation
	controllerConfig := newRestConfig(t)
	controllerConfig.Impersonate = rest.ImpersonationConfig{
		UserName: "system:serviceaccount:kube-system:replicaset-controller",
		Groups: []string{
			"system:serviceaccounts", "system:serviceaccounts:kube-system",
		},
	}
	controllerClient := kubernetes.NewForConfigOrDie(controllerConfig)

	namespace := "psp-test"
	_, err := adminClient.CoreV1().Namespaces().Create(&v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: namespace},
	})
	if err != nil {
		t.Fatalf("create namespace: %v", err)
	}
	defer adminClient.CoreV1().Namespaces().Delete(namespace, &metav1.DeleteOptions{})

	// Controller should not be able to create a privileged pod.
	err = retry(10, 1*time.Second, func() error {
		pod := newPrivilegedPod(namespace)
		_, err := controllerClient.CoreV1().Pods(namespace).Create(pod)
		if err == nil {
			t.Errorf("was able to create privileged pod")
			return nil
		}
		if apierrors.IsForbidden(err) {
			return nil
		}
		return fmt.Errorf("unexpected error: %v", err)
	})
	if err != nil {
		t.Errorf("creating privleged pod: %v", err)
	}

	// This binding lets the controller create a privileged pod in the namespace.
	binding := &v1beta1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "psp-permissive",
			Namespace: namespace,
		},
		Subjects: []v1beta1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "replicaset-controller",
				Namespace: "kube-system",
			},
		},
		RoleRef: v1beta1.RoleRef{
			Kind:     "ClusterRole",
			Name:     "psp-permissive",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	_, err = adminClient.RbacV1beta1().RoleBindings(namespace).Create(binding)
	if err != nil {
		t.Fatalf("create role binding: %v", err)
	}

	// Controller should now be able to create a privileged pod.
	err = retry(20, 1*time.Second, func() error {
		pod := newPrivilegedPod(namespace)
		_, err := controllerClient.CoreV1().Pods(namespace).Create(pod)
		if err == nil {
			return nil
		}
		if apierrors.IsForbidden(err) {
			// RBAC isn't instantaneous since it's driven by an informer.
			return err
		}
		return fmt.Errorf("unexpected error: %v", err)
	})
	if err != nil {
		t.Errorf("creating privleged pod: %v", err)
	}
}

func TestPSPCreateDeploymentInDefaultNS(t *testing.T) {
	adminConfig := newRestConfig(t)
	adminClient := kubernetes.NewForConfigOrDie(adminConfig)

	name := "redis"
	namespace := "psp-test-2"
	_, err := adminClient.CoreV1().Namespaces().Create(&v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: namespace},
	})
	if err != nil {
		t.Fatalf("create namespace: %v", err)
	}
	defer adminClient.CoreV1().Namespaces().Delete(namespace, &metav1.DeleteOptions{})

	deployment := newDeployment(name, namespace, "redis")
	if _, err := adminClient.AppsV1().Deployments(namespace).Create(deployment); err != nil {
		t.Logf("got error creating deployment: %v", err)
	}
	if err = retry(10, 1*time.Second, func() error {
		d, err := adminClient.AppsV1().Deployments(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			t.Logf("got error getting the deployment: %v", err)
			return err
		}
		// failue condition
		if d.Status.UnavailableReplicas < 1 {
			err := fmt.Errorf("unavailable replica is less than 1")
			t.Log(err)
			return err
		}
		return nil
	}); err != nil {
		t.Error(err)
	}

	namespace = "default"
	deployment = newDeployment(name, namespace, "redis")
	if _, err := adminClient.AppsV1().Deployments(namespace).Create(deployment); err != nil {
		t.Logf("got error creating deployment: %v", err)
	}
	if err = retry(10, 1*time.Second, func() error {
		d, err := adminClient.AppsV1().Deployments(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			t.Logf("got error getting the deployment: %v", err)
			return err
		}
		// failue condition
		if d.Status.UpdatedReplicas < 1 {
			err := fmt.Errorf("available replica is less than 1")
			t.Log(err)
			return err
		}

		return nil
	}); err != nil {
		t.Error(err)
	}
	defer adminClient.AppsV1().Deployments(namespace).Delete(name, &metav1.DeleteOptions{})
}
