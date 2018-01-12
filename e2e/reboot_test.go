package e2e

import (
	"fmt"
	"sort"
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
	if err := controlPlaneReady(client); err != nil {
		t.Fatalf("waiting for control plane: %v", err)
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

const checkpointAnnotation = "checkpointer.alpha.coreos.com/checkpoint-of"

// controlPlaneReady waits for API server availability and no checkpointed pods
// in kube-system.
func controlPlaneReady(c kubernetes.Interface) error {
	return retry(60, 5*time.Second, func() error {
		pods, err := c.CoreV1().Pods("kube-system").List(metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("get pods in kube-system: %v", err)
		}

		// list of pods that are checkpoint pods, not the real pods.
		var (
			checkpointPods []string
			regularPods    []string
		)
		for _, pod := range pods.Items {
			if len(pod.Annotations) > 0 && pod.Annotations[checkpointAnnotation] != "" {
				checkpointPods = append(checkpointPods, pod.Name)
			} else {
				regularPods = append(regularPods, pod.Name)
			}
		}
		if len(checkpointPods) > 0 {
			sort.Strings(checkpointPods)
			sort.Strings(regularPods)
			return fmt.Errorf("kube-system still has checkpoint pods=%q, non-checkpoint pods=%q", checkpointPods, regularPods)
		}
		return nil
	})
}
