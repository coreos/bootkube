package framework

import (
	"fmt"
	"os"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/tools/clientcmd"
)

// Cluster represents an interface to test bootkube based kubernetes clusters.
// The simplest way to write tests against it is to just write go tests that call
// out to NewCluster at the beginning of each test. External tooling should first
// provision the kubernetes cluster and output a kubeconfig.
type Cluster struct {
	Client kubernetes.Clientset

	expectedNodes int
}

func NewCluster() *Cluster {
	// uses the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	c := &Cluster{
		Client:        *clientset,
		expectedNodes: *expectedNodes,
	}

	return c
}

// Ready blocks until a Cluster is considered available. The current
// implementation checks that the expected number of nodes are registered.
func (c *Cluster) Ready() error {
	f := func() error {
		list, err := c.Client.CoreV1().Nodes().List(metav1.ListOptions{})
		if err != nil {
			return err
		}

		if len(list.Items) != c.expectedNodes {
			return fmt.Errorf("cluster is not ready, expected %v nodes got %v", c.expectedNodes, len(list.Items))
		}

		for _, node := range list.Items {
			if node.Status.Phase != v1.NodeRunning {
				return fmt.Errorf("One or more nodes not in the ready state: %v", node.Status.Phase)
			}
		}

		return nil
	}

	if err := Retry(12, 10*time.Second, f); err != nil {
		return err
	}
	return nil
}
