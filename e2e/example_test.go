package e2e

import (
	"testing"

	"github.com/kubernetes-incubator/bootkube/e2e/framework"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestExample(t *testing.T) {
	c := framework.NewCluster()

	podlist, err := c.Client.CoreV1().Pods("kube-system").List(metav1.ListOptions{})
	if err != nil {
		t.Fatalf("%v", err)
	}
	for _, pod := range podlist.Items {
		t.Logf(pod.ObjectMeta.Name)
	}
}
