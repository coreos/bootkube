package e2e

import (
	"fmt"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

func TestDeleteAPI(t *testing.T) {
	apiPods, err := client.CoreV1().Pods("kube-system").List(metav1.ListOptions{LabelSelector: "k8s-app=kube-apiserver"})
	if err != nil {
		t.Fatal(err)
	}

	// delete any api-server pods
	deletedPods := make(map[string]struct{})
	if err := wait.Poll(5*time.Second, 12*time.Minute, func() (bool, error) {
		for _, pod := range apiPods.Items {
			if _, isDeleted := deletedPods[pod.ObjectMeta.Name]; !isDeleted {
				now := int64(2)
				err := client.CoreV1().Pods("kube-system").Delete(pod.ObjectMeta.Name, &metav1.DeleteOptions{
					GracePeriodSeconds: &now,
				})
				if err != nil {
					if errors.IsNotFound(err) {
						t.Logf("Object does not exist")
						continue
					}
					t.Logf("Client error: %v", err)
					return false, err
				}
				deletedPods[pod.ObjectMeta.Name] = struct{}{}
			}
		}
		return len(deletedPods) == len(apiPods.Items), nil
	}); err != nil {
		t.Errorf("deletion of api-server pods failed: %v", err)
	}

	// wait for pods to be completely deleted.
	if err := retry(100, 1*time.Second, func() error {
		remainingPods, err := client.CoreV1().Pods("kube-system").List(metav1.ListOptions{LabelSelector: "k8s-app=kube-apiserver"})
		if err != nil {
			return fmt.Errorf("error checking for remaining apiserver pods: %v", err)
		}
		for _, pod := range remainingPods.Items {
			if _, ok := deletedPods[pod.ObjectMeta.Name]; ok {
				return fmt.Errorf("pod %s is still not deleted", pod.ObjectMeta.Name)
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("error waiting for apiserver pods to be deleted: %v", err)
	}

	// wait until api server is back up
	if err := controlPlaneReady(client, 120, 5*time.Second); err != nil {
		t.Fatalf("waiting for control plane: %v", err)
	}
}
