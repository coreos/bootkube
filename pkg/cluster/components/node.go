package components

import (
	"fmt"
	"time"

	"github.com/kubernetes-incubator/bootkube/pkg/cluster/components/version"
	"github.com/kubernetes-incubator/bootkube/pkg/node"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/util/wait"
)

const (
	nodePriority      = 1000
	nodeUpdateTimeout = 2 * time.Minute
)

// NodeUpdater is responsible for updating nodes via
// annotations which are then handled by the node-agent.
type NodeUpdater struct {
	// client is an API Server client.
	client unversioned.Interface
	// nodeStore is cache of node backed by an informer.
	nodes cache.StoreToNodeLister
	// node is the node this updater is responsible for.
	node *api.Node
}

func NewNodeUpdater(client unversioned.Interface, node *api.Node, nodes cache.StoreToNodeLister) (*NodeUpdater, error) {
	return &NodeUpdater{
		client: client,
		nodes:  nodes,
		node:   node,
	}, nil
}

func (nu *NodeUpdater) Name() string {
	return nu.node.Name
}

// Priority for Nodes should be such that they
// get updated last, and rolled back first.
func (nu *NodeUpdater) Priority() int {
	return nodePriority
}

// Version returns the highest version of any
// Node in the store.
func (nu *NodeUpdater) Version() (*version.Version, error) {
	ni, exists, err := nu.nodes.Get(nu.node)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("could not find Node %s in store", nu.Name())
	}
	n, ok := ni.(*api.Node)
	if !ok {
		return nil, fmt.Errorf("unexpected type returned from Node store, expected *v1.Node, got %T", ni)
	}
	versionString, ok := n.Annotations[node.CurrentVersionAnnotation]
	if !ok {
		return nil, fmt.Errorf("no version annotation for Node %s", n.Name)
	}
	return version.ParseFromImageString(versionString)
}

// UpdateToVersion will update the Node to the given version.
func (nu *NodeUpdater) UpdateToVersion(v *version.Version) (bool, error) {
	n, err := nu.client.Nodes().Get(nu.Name())
	if err != nil {
		return false, err
	}
	// First step: update the annotation on the Node object. This will
	// trigger the node-agent running on that node to update the Node.
	if n.Annotations == nil {
		n.Annotations = make(map[string]string)
	}
	if n.Annotations[node.CurrentVersionAnnotation] == v.ImageString() {
		return false, nil
	}
	n.Annotations[node.DesiredVersionAnnotation] = v.ImageString()
	_, err = nu.client.Nodes().Update(n)
	if err != nil {
		return false, err
	}
	// Second step: wait until the node-agent has updated the Node.
	err = wait.Poll(time.Second, nodeUpdateTimeout, func() (bool, error) {
		v, ok, err := nu.nodes.Get(n)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, fmt.Errorf("unable to find Node %s in store", n.Name)
		}
		nn := v.(*api.Node)
		if nn.Annotations[node.DesiredVersionAnnotation] == nn.Annotations[node.CurrentVersionAnnotation] {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return false, err
	}
	return true, nil
}
