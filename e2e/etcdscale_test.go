package e2e

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
)

func TestEtcdScale(t *testing.T) {
	// TODO(etcd-team): re-enable this once scaling is stable.
	t.Skip("disabled due to flakiness")

	// check that we have 3 or more masters
	etcdScalePreCheck(client, t)

	// scale up etcd operator
	if err := resizeSelfHostedEtcd(client, 3); err != nil {
		t.Fatalf("scaling up: %v", err)
	}

	// check that each pod runs on a different master node
	if err := checkEtcdPodDistribution(client, 3); err != nil {
		t.Fatal(err)
	}

	// scale back to 1
	if err := resizeSelfHostedEtcd(client, 1); err != nil {
		t.Fatalf("scaling down: %v", err)
	}
}

// Skip if not running 3 or more master nodes unless explicitly told to be
// expecting 3 or more. Then block until 3 are ready or fail. Also check that
// etcd is self-hosted.
func etcdScalePreCheck(c kubernetes.Interface, t *testing.T) {
	var requiredMasters int = 3
	if expectedMasters < requiredMasters {
		t.Skipf("Test requires %d masters, test was run expecting %d", requiredMasters, expectedMasters)
	}

	checkMasters := func() error {
		listOpts := metav1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/master",
		}
		list, err := c.CoreV1().Nodes().List(listOpts)
		if err != nil {
			return fmt.Errorf("error listing nodes: %v", err)
		}
		if len(list.Items) < requiredMasters {
			return fmt.Errorf("not enough master nodes for etcd scale test: %v", len(list.Items))
		}
		var ready int = 0
		for _, node := range list.Items {
			for _, condition := range node.Status.Conditions {
				if condition.Type == v1.NodeReady && condition.Status == v1.ConditionTrue {
					ready++
				}
			}
		}
		if ready < requiredMasters {
			return fmt.Errorf("not enough master nodes are ready for etcd scale test: need %d, ready: %d", requiredMasters, ready)
		}

		return nil
	}
	if err := retry(50, 10*time.Second, checkMasters); err != nil {
		t.Fatal(err)
	}

	// check for etcd-operator by getting pod
	l, err := c.CoreV1().Pods("kube-system").List(metav1.ListOptions{LabelSelector: "k8s-app=etcd-operator"})
	if err != nil || len(l.Items) == 0 {
		t.Fatalf("test requires a cluster with self-hosted etcd: %v", err)
	}
}

const kubeEtcdTPRURI = "/apis/etcd.coreos.com/v1beta1/namespaces/kube-system/clusters/kube-etcd"

// resizes self-hosted etcd and checks that the desired number of pods are in a running state
func resizeSelfHostedEtcd(c kubernetes.Interface, size int) error {
	var tpr unstructured.Unstructured

	// get tpr
	httpRestClient := c.ExtensionsV1beta1().RESTClient()
	b, err := httpRestClient.Get().RequestURI(kubeEtcdTPRURI).DoRaw()
	if err != nil {
		return err
	}

	if err := json.Unmarshal(b, &tpr); err != nil {
		return fmt.Errorf("failed to unmarshal TPR: %v", err)
	}

	// change size
	spec, ok := tpr.Object["spec"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("could not get 'spec' from TPR")
	}
	spec["size"] = size

	// update tpr
	if err := updateEtcdTPR(c, &tpr); err != nil {
		return err
	}

	// check that all pods are running by checking TPR
	podsReady := func() error {
		// get tpr
		httpRestClient := c.ExtensionsV1beta1().RESTClient()
		b, err := httpRestClient.Get().RequestURI(kubeEtcdTPRURI).DoRaw()
		if err != nil {
			return err
		}

		if err := json.Unmarshal(b, &tpr); err != nil {
			return fmt.Errorf("failed to unmarshal TPR: %v", err)
		}

		// check status of members
		status, ok := tpr.Object["status"].(map[string]interface{})
		if !ok {
			return fmt.Errorf("could not asset 'status' type from TPR")
		}
		members, ok := status["members"].(map[string]interface{})
		if !ok {
			return fmt.Errorf("could not assert 'members' type from TPR")
		}
		readyList, ok := members["ready"].([]interface{})
		if !ok {
			return fmt.Errorf("could not assert 'ready' type from TPR")
		}

		// check that we have enough nodes considered ready by operator
		if len(readyList) != size {
			return fmt.Errorf("expected %d etcd pods got %d: %v", size, len(readyList), readyList)
		}

		return nil
	}

	if err := retry(31, 10*time.Second, podsReady); err != nil {
		return fmt.Errorf("Waited 300 seconds for etcd to scale: %v", err)
	}

	return nil
}

func updateEtcdTPR(c kubernetes.Interface, tpr *unstructured.Unstructured) error {
	data, err := json.Marshal(tpr)
	if err != nil {
		return err
	}

	var statusCode int

	httpRestClient := c.ExtensionsV1beta1().RESTClient()
	result := httpRestClient.Put().RequestURI(kubeEtcdTPRURI).Body(data).Do()

	if result.Error() != nil {
		return result.Error()
	}

	result.StatusCode(&statusCode)

	if statusCode != 200 {
		return fmt.Errorf("unexpected status code %d, expecting 200", statusCode)
	}

	return nil
}

// Checks that self-hosted etcd pods are scheduled on different master nodes
// when possible. Look at the number of unique nodes etcd pods are scheduled
// on and compare to the lesser value between total number of master nodes and
// total number of etcd pods.
func checkEtcdPodDistribution(c kubernetes.Interface, etcdClusterSize int) error {
	// get pods
	pods, err := client.CoreV1().Pods("kube-system").List(metav1.ListOptions{LabelSelector: "etcd_cluster=kube-etcd"})
	if err != nil || len(pods.Items) != etcdClusterSize {
		return fmt.Errorf("getting etcd pods err: %v || %v != %v", err, len(pods.Items), etcdClusterSize)
	}
	// get master nodes
	mnodes, err := c.CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/master"})
	if err != nil {
		return fmt.Errorf("error listing nodes: %v", err)
	}

	// set of nodes pods are running on identified by HostIP
	nodeSet := map[string]struct{}{}
	for _, pod := range pods.Items {
		nodeSet[pod.Status.HostIP] = struct{}{}
	}

	var expectedUniqueNodes int
	if len(mnodes.Items) > etcdClusterSize {
		expectedUniqueNodes = etcdClusterSize
	} else {
		expectedUniqueNodes = len(mnodes.Items)
	}

	if len(nodeSet) != expectedUniqueNodes {
		return fmt.Errorf("self-hosted etcd pods not properly distributed")
	}

	// check that each node in nodeSet is a master node
	masterSet := map[string]struct{}{}
	for _, node := range mnodes.Items {
		for _, addr := range node.Status.Addresses {
			if addr.Type == v1.NodeInternalIP {
				masterSet[addr.Address] = struct{}{}
				break
			}
		}
	}

	for k := range nodeSet {
		if _, ok := masterSet[k]; !ok {
			return fmt.Errorf("detected self-hosted etcd pod running on non-master node %v %v", masterSet, nodeSet)
		}
	}

	return nil
}
