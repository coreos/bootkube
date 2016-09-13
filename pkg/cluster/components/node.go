package components

import (
	"errors"
	"fmt"
	"time"

	"github.com/coreos/bootkube/pkg/node"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/client/cache"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/release_1_3"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/watch"
)

// NodeUpdater is responsible for updating nodes via
// annotations which are then handled by the node-agent.
type NodeUpdater struct {
	// client is an API Server client.
	client clientset.Interface
	// nodeStore is cache of node backed by an informer.
	nodeStore cache.Store
}

func NewNodeUpdater(client clientset.Interface, selector api.ListOptions) (*NodeUpdater, error) {
	nodeStore, nodeController := framework.NewInformer(
		&cache.ListWatch{
			ListFunc: func(lo api.ListOptions) (runtime.Object, error) {
				return client.Core().Nodes().List(selector)
			},
			WatchFunc: func(lo api.ListOptions) (watch.Interface, error) {
				return client.Core().Nodes().Watch(selector)
			},
		},
		&v1.Pod{},
		30*time.Minute,
		framework.ResourceEventHandlerFuncs{},
	)
	go nodeController.Run(wait.NeverStop)
	return &NodeUpdater{
		client:    client,
		nodeStore: nodeStore,
	}, nil
}

func (nu *NodeUpdater) Name() string {
	return "nodes"
}

// CurrentVersion returns the current version of the Nodes we are
// managing. We pick the lowest version so that if we encountered
// an error during the last update we can pick up where we left off.
func (nu *NodeUpdater) CurrentVersion() (*Version, error) {
	var v *Version
	for _, ni := range nu.nodeStore.List() {
		n := ni.(*v1.Node)
		if n.Annotations == nil {
			return nil, fmt.Errorf("no annotations for Node %s", n.Name)
		}
		ver, err := ParseVersionFromImage(n.Annotations[node.CurrentVersionAnnotation])
		if err != nil {
			return nil, err
		}
		if v == nil {
			v = ver
		} else if v.Semver.GT(ver.Semver) {
			v = ver
		}
	}
	if v == nil {
		return nil, errors.New("unable to get current version for Nodes")
	}
	return v, nil
}

// UpdateToVersion will update the Node to the given version.
func (nu *NodeUpdater) UpdateToVersion(client clientset.Interface, v *Version) error {
	nodes := nu.nodeStore.List()
	for _, ni := range nodes {
		n := ni.(*v1.Node)
		// First step: update the annotation on the Node object. This will
		// trigger the node-agent running on that node to update the Node.
		if n.Annotations == nil {
			n.Annotations = make(map[string]string)
		}
		if n.Annotations[node.CurrentVersionAnnotation] == v.Image.String() {
			continue
		}
		n.Annotations[node.DesiredVersionAnnotation] = v.Image.String()
		_, err := nu.client.Core().Nodes().Update(n)
		if err != nil {
			return err
		}
		// Second step: wait until the node-agent has updated the Node.
		err = wait.Poll(time.Second, 10*time.Minute, func() (bool, error) {
			v, ok, err := nu.nodeStore.Get(n)
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
