package components

import (
	"fmt"
	"time"

	"github.com/coreos/bootkube/pkg/node"
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
}

func NewNodeUpdater(client clientset.Interface, nodes cache.StoreToNodeLister) (*NodeUpdater, error) {
	return &NodeUpdater{
		client: client,
		nodes:  nodes,
	}, nil
}

func (nu *NodeUpdater) Name() string {
	return "nodes"
}

// Node priority is not used, Nodes are always
// updated last.
func (nu *NodeUpdater) Priority() int {
	return 0
}

// UpdateToVersion will update the Node to the given version.
func (nu *NodeUpdater) UpdateToVersion(v *Version) error {
	nl, err := nu.nodes.List()
	if err != nil {
		return err
	}
	for _, n := range nl.Items {
		// First step: update the annotation on the Node object. This will
		// trigger the node-agent running on that node to update the Node.
		if n.Annotations == nil {
			n.Annotations = make(map[string]string)
		}
		if n.Annotations[node.CurrentVersionAnnotation] == v.Image.String() {
			continue
		}
		n.Annotations[node.DesiredVersionAnnotation] = v.Image.String()
		var out v1.Node
		v1.Convert_api_Node_To_v1_Node(&n, &out, nil)
		_, err := nu.client.Core().Nodes().Update(&out)
		if err != nil {
			return err
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
			return err
		}
	}
	return nil
}
