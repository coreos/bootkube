package utils

import (
	"time"

	"k8s.io/client-go/pkg/api/v1"
)

const (
	// NodeRoleMasterLabel defines the master node's label.
	NodeRoleMasterLabel = "node-role.kubernetes.io/master"
	// NodeRoleWorkerLabel defines the worker node's label.
	NodeRoleWorkerLabel = "node-role.kubernetes.io/node"
)

// Retry retries f until f return nil error.
// It makes max attempts and adds delay between each attempt.
func Retry(attempts int, delay time.Duration, f func() error) error {
	var err error

	for i := 0; i < attempts; i++ {
		err = f()
		if err == nil {
			break
		}

		if i < attempts-1 {
			time.Sleep(delay)
		}
	}

	return err
}

// IsMaster returns true if the node's labels contains "node-role.kubernetes.io/master".
func IsMaster(n *v1.Node) bool {
	_, ok := n.Labels[NodeRoleMasterLabel]
	return ok
}

// IsWorker returns true if the node's labels contains "node-role.kubernetes.io/node".
func IsWorker(n *v1.Node) bool {
	_, ok := n.Labels[NodeRoleWorkerLabel]
	return ok
}

// ExternalIP returns external IP for a node.
// Will be empty string if not External IP found for node.
func ExternalIP(n *v1.Node) string {
	var host string
	for _, addr := range n.Status.Addresses {
		if addr.Type == v1.NodeExternalIP {
			host = addr.Address
			break
		}
	}
	return host
}
