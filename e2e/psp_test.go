package e2e

import (
	"fmt"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/apis/rbac/v1beta1"
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
	err = retry(10, 1*time.Second, func() error {
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
