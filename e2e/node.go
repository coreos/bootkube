package e2e

import (
	"bytes"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/api/v1"

	"golang.org/x/crypto/ssh"
)

const (
	LabelNodeRoleMaster = "node-role.kubernetes.io/master"
)

type Node struct {
	*v1.Node
}

func newNode(n *v1.Node) *Node {
	return &Node{n}
}

func (n *Node) GetIPByType(addrType v1.NodeAddressType) string {
	var host string
	for _, addr := range n.Status.Addresses {
		if addr.Type == addrType {
			host = addr.Address
			break
		}
	}
	return host
}

func (n *Node) ExternalIP() string {
	return n.GetIPByType(v1.NodeExternalIP)
}

func (n *Node) InternalIP() string {
	return n.GetIPByType(v1.NodeInternalIP)
}

func (n *Node) SSH(cmd string) (stdout, stderr []byte, err error) {
	host := n.ExternalIP()
	if host == "" {
		host = n.InternalIP()
		if host == "" {
			return nil, nil, fmt.Errorf("cannot find external or internal IP for node %q", n.Name)
		}
	}
	return sshClient.SSH(host, cmd)
}

func (n *Node) Reboot() error {

	// ssh to node and reboot
	rebooter := func() error {
		stdout, stderr, err := n.SSH("sudo reboot")
		if _, ok := err.(*ssh.ExitMissingError); ok {
			// A terminated session is perfectly normal during reboot.
			err = nil
		}
		if err != nil {
			return fmt.Errorf("issuing reboot command failed: %v\nstdout:%s\nstderr:%s", err, stdout, stderr)
		}
		return err
	}

	// ensure rebooted node is running
	checker := func() error {
		stdout, stderr, err := n.SSH("systemctl is-system-running")
		if err != nil {
			return fmt.Errorf("%v: %v", err, stderr)
		}
		if !bytes.Contains(stdout, []byte("running")) {
			return fmt.Errorf("system is not running yet")
		}
		return nil
	}

	if err := retry(5, 30*time.Second, rebooter); err != nil {
		return err
	}

	return retry(20, 10*time.Second, checker)
}

// IsMaster returns true if the node's labels contains "node-role.kubernetes.io/master".
func (n *Node) IsMaster() bool {
	_, ok := n.Labels[LabelNodeRoleMaster]
	return ok
}

// Cluster is a simple abstraction to make writing tests easier.
type Cluster struct {
	Masters []*Node
	Workers []*Node
}

// GetCluster can be called in every test to return a *Cluster object.
func GetCluster() (*Cluster, error) {
	var c Cluster

	nodelist, err := client.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for i := range nodelist.Items {
		nn := newNode(&nodelist.Items[i])
		if nn.IsMaster() {
			c.Masters = append(c.Masters, nn)
		} else {
			c.Workers = append(c.Workers, nn)
		}
	}
	return &c, nil
}
