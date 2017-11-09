package e2e

import (
	"fmt"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDeleteAPI(t *testing.T) {
	apiPods, err := client.CoreV1().Pods("kube-system").List(metav1.ListOptions{LabelSelector: "k8s-app=kube-apiserver"})
	if err != nil {
		t.Fatal(err)
	}

	// delete any api-server pods
	for i, pod := range apiPods.Items {
		err := client.CoreV1().Pods("kube-system").Delete(pod.ObjectMeta.Name, &metav1.DeleteOptions{})
		if err != nil {
			// TODO: if HA we should be able to successfully
			// delete all. Until then just log if we can't delete
			// something that isn't the first pod.
			if i == 0 {
				t.Fatalf("error deleting api-server pod: %v", err)
			} else {
				t.Logf("error deleting api-server pod: %v", err)
			}
		}
	}

	// wait for api-server to go down by waiting until listing pods returns
	// errors. This is potentially error prone, but without waiting for the
	// apiserver to go down the next step will return sucess before the
	// apiserver is ever destroyed.
	waitDestroy := func() error {
		// only checking api being down , specific function not important
		_, err := client.Discovery().ServerVersion()

		if err == nil {
			return fmt.Errorf("waiting for apiserver to go down: %v", err)
		}
		return nil
	}

	if err := retry(100, 500*time.Millisecond, waitDestroy); err != nil {
		t.Fatal(err)
	}

	// wait until api server is back up
	waitAPI := func() error {
		// only checking for presence of api returning, specific function not important
		_, err := client.Discovery().ServerVersion()
		if err != nil {
			return fmt.Errorf("waiting for apiserver to return: %v", err)
		}
		return nil
	}
	if err := retry(30, 10*time.Second, waitAPI); err != nil {
		t.Fatal(err)
	}
}
