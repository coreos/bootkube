package components

import (
	"fmt"
	"time"

	"github.com/coreos/bootkube/pkg/node"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/client/cache"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/release_1_3"
	"k8s.io/kubernetes/pkg/util/wait"
)

// NodeUpdater is responsible for updating nodes via
// annotations which are then handled by the node-agent.
type NodeUpdater struct {
	// client is an API Server client.
	client clientset.Interface
	// nodeStore is cache of node backed by an informer.
	nodes cache.StoreToNodeLister
	// node is the node this updater is responsible for.
	node *api.Node
}

func NewNodeUpdater(client clientset.Interface, node *api.Node, nodes cache.StoreToNodeLister) (*NodeUpdater, error) {
	return &NodeUpdater{
		client: client,
		nodes:  nodes,
		node:   node,
	}, nil
}

func (nu *NodeUpdater) Name() string {
	return nu.node.Name
}

// Node priority is not used, Nodes are always
// updated last.
func (nu *NodeUpdater) Priority() int {
	return 0
}

// Version returns the highest version of any
// Node in the store.
func (nu *NodeUpdater) Version() (*Version, error) {
	nl, err := nu.nodes.List()
	if err != nil {
		return nil, err
	}
	var highest *Version
	for _, n := range nl.Items {
		versionString, ok := n.Annotations[node.CurrentVersionAnnotation]
		if !ok {
			return nil, fmt.Errorf("no version annotation for Node %s", n.Name)
		}
		v, err := ParseVersionFromImage(versionString)
		if err != nil {
			return nil, err
		}
		if highest == nil {
			highest = v
			continue
		}
		if v.Semver().GT(highest.Semver()) {
			highest = v
		}
	}
	return highest, nil
}

// UpdateToVersion will update the Node to the given version.
func (nu *NodeUpdater) UpdateToVersion(v *Version) (bool, error) {
	ni, exists, err := nu.nodes.Get(nu.node)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, fmt.Errorf("Node %s does not exist in local store", nu.node.Name)
	}
	n, ok := ni.(*v1.Node)
	if !ok {
		return false, fmt.Errorf("got incorrect type from node store, expected *v1.Node, got %t", n)
	}
	// First step: update the annotation on the Node object. This will
	// trigger the node-agent running on that node to update the Node.
	if n.Annotations == nil {
		n.Annotations = make(map[string]string)
	}
	if n.Annotations[node.CurrentVersionAnnotation] == v.image.String() {
		return false, nil
	}
	n.Annotations[node.DesiredVersionAnnotation] = v.image.String()
	_, err = nu.client.Core().Nodes().Update(n)
	if err != nil {
		return false, err
	}
	// Second step: wait until the node-agent has updated the Node.
	err = wait.Poll(time.Second, 10*time.Minute, func() (bool, error) {
		v, ok, err := nu.nodes.Get(n)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, fmt.Errorf("unable to find Node %s in store", n.Name)
		}
		nn := v.(*v1.Node)
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
