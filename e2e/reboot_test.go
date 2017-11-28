package e2e

import (
	"fmt"
	"sync"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
)

// Reboot all nodes in cluster all at once. Wait for nodes to return. Run nginx
// workload.
func TestReboot(t *testing.T) {
	nodeList, err := client.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("rebooting %v nodes", len(nodeList.Items))

	var wg sync.WaitGroup
	for _, node := range nodeList.Items {
		wg.Add(1)
		go func(node v1.Node) {
			defer wg.Done()
			if err := newNode(&node).Reboot(); err != nil {
				t.Errorf("failed to reboot node: %v", err)
			}
		}(node)
	}
	wg.Wait()

	if err := nodesReady(client, nodeList, t); err != nil {
		t.Fatalf("some or all nodes did not recover from reboot: %v", err)
	}
}

// nodesReady blocks until all nodes in list are ready based on Name. Safe
// against new unknown nodes joining while the original set reboots.
func nodesReady(c kubernetes.Interface, expectedNodes *v1.NodeList, t *testing.T) error {
	var expectedNodeSet = make(map[string]struct{})
	for _, node := range expectedNodes.Items {
		expectedNodeSet[node.ObjectMeta.Name] = struct{}{}
	}

	return retry(80, 5*time.Second, func() error {
		list, err := c.CoreV1().Nodes().List(metav1.ListOptions{})
		if err != nil {
			return err
		}
		var recoveredNodes int
		for _, node := range list.Items {
			_, ok := expectedNodeSet[node.ObjectMeta.Name]
			if !ok {
				t.Logf("unexpected node checked in")
				continue
			}

			for _, condition := range node.Status.Conditions {
				if condition.Type == v1.NodeReady {
					if condition.Status == v1.ConditionTrue {
						recoveredNodes++
					} else {
						return fmt.Errorf("one or more nodes not in the ready state: %v", node.Status.Phase)
					}
					break
				}
			}
		}
		if recoveredNodes != len(expectedNodeSet) {
			return fmt.Errorf("not enough nodes recovered, expected %v got %v", len(expectedNodeSet), recoveredNodes)
		}
		return nil
	})
}
