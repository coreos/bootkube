package e2e

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

var nginxDS = []byte(`apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: nginx-daemonset
spec:
  template:
    metadata:
      labels:
        app: nginx-checkpoint-test
      annotations:
        checkpointer.alpha.coreos.com/checkpoint: "true"
    spec:
      hostNetwork: true
      containers:
        - name: nginx
          image: nginx`)

func verifyCheckpoint(c *Cluster, namespace, daemonsetName string, shouldExist, shouldBeActive bool) error {
	checkpointed := func() error {
		dirs := []string{
			"/etc/kubernetes/inactive-manifests/",
			"/etc/kubernetes/checkpoint-secrets/" + namespace,
			// TODO(yifan): Add configmap.
		}

		if shouldBeActive {
			dirs = append(dirs, "/etc/kubernetes/manifests")
		}

		for _, dir := range dirs {
			stdout, stderr, err := c.Workers[0].SSH("sudo ls " + dir)
			if err != nil {
				return fmt.Errorf("unable to ls %q, error: %v\nstdout: %s\nstderr: %s", dir, err, stdout, stderr)
			}

			if shouldExist && !bytes.Contains(stdout, []byte(daemonsetName)) {
				return fmt.Errorf("unable to find checkpoint %q in %q: error: %v, output: %q", daemonsetName, dir, err, stdout)
			}
			if !shouldExist && bytes.Contains(stdout, []byte(daemonsetName)) {
				return fmt.Errorf("should not find checkpoint %q in %q, error: %v, output: %q", daemonsetName, dir, err, stdout)
			}
		}

		// Check active checkpoints.
		dir := "/etc/kubernetes/manifests"
		stdout, stderr, err := c.Workers[0].SSH("sudo ls " + dir)
		if err != nil {
			return fmt.Errorf("unable to ls %q, error: %v\nstdout: %s\nstderr: %s", dir, err, stdout, stderr)
		}
		if shouldBeActive && !bytes.Contains(stdout, []byte(daemonsetName)) {
			return fmt.Errorf("unable to find checkpoint %q in %q: error: %v, output: %q", daemonsetName, dir, err, stdout)
		}
		if !shouldBeActive && bytes.Contains(stdout, []byte(daemonsetName)) {
			return fmt.Errorf("should not find checkpoint %q in %q, error: %v, output: %q", daemonsetName, dir, err, stdout)
		}

		return nil
	}
	return retry(60, 10*time.Second, checkpointed)
}

func verifyPod(c *Cluster, daemonsetName string, shouldRun bool) error {
	checkpointsRunning := func() error {
		stdout, stderr, err := c.Workers[0].SSH("docker ps")
		if err != nil {
			return fmt.Errorf("unable to docker ps, error: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
		}

		if shouldRun && !bytes.Contains(stdout, []byte(daemonsetName)) {
			return fmt.Errorf("unable to find running checkpoints %q, error: %v, output: %q", daemonsetName, err, stdout)
		}
		if !shouldRun && bytes.Contains(stdout, []byte(daemonsetName)) {
			return fmt.Errorf("should not find running checkpoints %q, error: %v, output: %q", daemonsetName, err, stdout)
		}
		return nil
	}
	return retry(60, 10*time.Second, checkpointsRunning)
}

func isNodeReady(n *Node) bool {
	for _, condition := range n.Status.Conditions {
		if condition.Type == v1.NodeReady && condition.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

// waitCluster waits for master and workers to be ready.
func waitCluster(t *testing.T) *Cluster {
	var c *Cluster
	var err error

	f := func() error {
		c, err = GetCluster()
		if err != nil {
			t.Fatalf("Failed to get cluster")
		}
		if len(c.Masters) == 0 {
			return fmt.Errorf("no masters")
		}
		if len(c.Workers) == 0 {
			return fmt.Errorf("no workers")
		}
		for i := range c.Masters {
			if !isNodeReady(c.Masters[i]) {
				return fmt.Errorf("masters[%d] is not ready", i)
			}
		}
		for i := range c.Workers {
			if !isNodeReady(c.Workers[i]) {
				return fmt.Errorf("workers[%d] is not ready", i)
			}
		}
		return nil
	}

	if err := retry(60, 10*time.Second, f); err != nil {
		t.Fatalf("Failed to wait cluster: %v", err)
	}
	return c
}

// 1. Schedule a pod checkpointer on worker node.
// 2. Schedule a test pod on worker node.
// 3. Reboot the worker without starting the kubelet.
// 4. Delete the checkpointer on API server.
// 5. Reboot the masters without starting the kubelet.
// 6. Start the worker kubelet, verify the checkpointer and the pod are still running as static pods.
// 7. Start the master kubelets, verify the checkpointer is removed but the pod is still running.
func TestCheckpointerUnscheduleCheckpointer(t *testing.T) {
	// Get the cluster
	c := waitCluster(t)

	testNS := strings.ToLower(fmt.Sprintf("%s-%s", namespace, t.Name()))
	if _, err := createNamespace(client, testNS); err != nil {
		t.Fatalf("Failed to create namespace: %v", err)
	}
	defer deleteNamespace(client, testNS)

	// Run the pod checkpointer on worker nodes as well.
	patch := `[{"op": "replace", "path": "/spec/template/spec/nodeSelector", "value": {}}]`
	_, err := client.ExtensionsV1beta1().DaemonSets("kube-system").Patch("pod-checkpointer", types.JSONPatchType, []byte(patch))
	if err != nil {
		t.Fatalf("Failed to patch checkpointer: %v", err)
	}
	if err := verifyPod(c, "pod-checkpointer", true); err != nil {
		t.Fatalf("Failed to verifyPod: %s", err)
	}

	// Create test pod.
	obj, _, err := api.Codecs.UniversalDecoder().Decode(nginxDS, nil, &v1beta1.DaemonSet{})
	if err != nil {
		t.Fatalf("Unable to decode manifest: %v", err)
	}
	ds, ok := obj.(*v1beta1.DaemonSet)
	if !ok {
		t.Fatalf("Expected manifest to decode into *api.Daemonset, got %T", ds)
	}
	_, err = client.ExtensionsV1beta1().DaemonSets(testNS).Create(ds)
	if err != nil {
		t.Fatalf("Failed to create the checkpoint parent: %v", err)
	}
	if err := verifyPod(c, "nginx-daemonset", true); err != nil {
		t.Fatalf("Failed to verifyPod: %s", err)
	}

	// Verify the checkpoints are created.
	if err := verifyCheckpoint(c, "kube-system", "pod-checkpointer", true, true); err != nil {
		t.Fatalf("Failed to verify checkpoint: %v", err)
	}
	if err := verifyCheckpoint(c, testNS, "nginx-daemonset", true, false); err != nil {
		t.Fatalf("Failed to verify checkpoint: %v", err)
	}

	// Disable the kubelet and reboot the worker.
	stdout, stderr, err := c.Workers[0].SSH("sudo systemctl disable kubelet")
	if err != nil {
		t.Fatalf("Failed to disable kubelet on worker %q: %v\nstdout: %s\nstderr: %s", c.Workers[0].Name, err, stdout, stderr)
	}
	if err := c.Workers[0].Reboot(); err != nil {
		t.Fatalf("Failed to reboot worker: %v", err)
	}

	// Delete the pod checkpointer on the worker node by updating the daemonset.
	patch = `{"spec":{"template":{"spec":{"nodeSelector":{"node-role.kubernetes.io/master":""}}}}}`
	_, err = client.ExtensionsV1beta1().DaemonSets("kube-system").Patch("pod-checkpointer", types.MergePatchType, []byte(patch))
	if err != nil {
		t.Fatalf("Failed to patch checkpointer: %v", err)
	}

	// Disable the kubelet and reboot the masters.
	var rebootGroup sync.WaitGroup
	for i := range c.Masters {
		rebootGroup.Add(1)
		go func(i int) {
			defer rebootGroup.Done()
			stdout, stderr, err = c.Masters[i].SSH("sudo systemctl disable kubelet")
			if err != nil {
				t.Fatalf("Failed to disable kubelet on master %q: %v\nstdout: %s\nstderr: %s", c.Masters[i].Name, err, stdout, stderr)
			}
			if err := c.Masters[i].Reboot(); err != nil {
				t.Fatalf("Failed to reboot master: %v", err)
			}
		}(i)
	}
	rebootGroup.Wait()

	// Start the worker kubelet.
	stdout, stderr, err = c.Workers[0].SSH("sudo systemctl enable kubelet && sudo systemctl start kubelet")
	if err != nil {
		t.Fatalf("Failed to start kubelet on worker %q: %v\nstdout: %s\nstderr: %s", c.Workers[0].Name, err, stdout, stderr)
	}

	// Verify that the checkpoints are still running.
	if err := verifyPod(c, "pod-checkpointer", true); err != nil {
		t.Fatalf("Failed to verifyPod: %s", err)
	}
	if err := verifyPod(c, "nginx-daemonset", true); err != nil {
		t.Fatalf("Failed to verifyPod: %s", err)
	}

	// Start the master kubelet.
	var enableGroup sync.WaitGroup
	for i := range c.Masters {
		enableGroup.Add(1)
		go func(i int) {
			defer enableGroup.Done()
			stdout, stderr, err = c.Masters[i].SSH("sudo systemctl enable kubelet && sudo systemctl start kubelet")
			if err != nil {
				t.Fatalf("Failed to start kubelet on master %q: %v\nstdout: %s\nstderr: %s", c.Masters[i].Name, err, stdout, stderr)
			}
		}(i)
	}
	enableGroup.Wait()

	// Verify that the pod-checkpointer is cleaned up but the daemonset is still running.
	if err := verifyPod(c, "pod-checkpointer", false); err != nil {
		t.Fatalf("Failed to verifyPod: %s", err)
	}
	if err := verifyPod(c, "nginx-daemonset", true); err != nil {
		t.Fatalf("Failed to verifyPod: %s", err)
	}
	if err := verifyCheckpoint(c, "kube-system", "pod-checkpointer", false, false); err != nil {
		t.Fatalf("Failed to verifyCheckpoint: %s", err)
	}
	if err := verifyCheckpoint(c, testNS, "nginx-daemonset", false, false); err != nil {
		t.Fatalf("Failed to verifyCheckpoint: %s", err)
	}
}

// 1. Schedule a pod checkpointer on worker node.
// 2. Schedule a test pod on worker node.
// 3. Reboot the worker without starting the kubelet.
// 4. Delete the test pod on API server.
// 5. Reboot the masters without starting the kubelet.
// 6. Start the worker kubelet, verify the checkpointer and the pod are still running as static pods.
// 7. Start the master kubelets, verify the test pod is removed, but not the checkpointer.
func TestCheckpointerUnscheduleParent(t *testing.T) {
	// Get the cluster
	c := waitCluster(t)

	testNS := strings.ToLower(fmt.Sprintf("%s-%s", namespace, t.Name()))
	if _, err := createNamespace(client, testNS); err != nil {
		t.Fatalf("Failed to create namespace: %v", err)
	}
	defer deleteNamespace(client, testNS)

	// Run the pod checkpointer on worker nodes as well.
	patch := `[{"op": "replace", "path": "/spec/template/spec/nodeSelector", "value": {}}]`
	_, err := client.ExtensionsV1beta1().DaemonSets("kube-system").Patch("pod-checkpointer", types.JSONPatchType, []byte(patch))
	if err != nil {
		t.Fatalf("Failed to patch checkpointer: %v", err)
	}
	if err := verifyPod(c, "pod-checkpointer", true); err != nil {
		t.Fatalf("verifyPod: %s", err)
	}

	// Create test pod.
	obj, _, err := api.Codecs.UniversalDecoder().Decode(nginxDS, nil, &v1beta1.DaemonSet{})
	if err != nil {
		t.Fatalf("Unable to decode manifest: %v", err)
	}
	ds, ok := obj.(*v1beta1.DaemonSet)
	if !ok {
		t.Fatalf("Expected manifest to decode into *api.Daemonset, got %T", ds)
	}
	_, err = client.ExtensionsV1beta1().DaemonSets(testNS).Create(ds)
	if err != nil {
		t.Fatalf("Failed to create the checkpoint parent: %v", err)
	}
	if err := verifyPod(c, "nginx-daemonset", true); err != nil {
		t.Fatalf("verifyPod: %s", err)
	}

	// Verify the checkpoints are created.
	if err := verifyCheckpoint(c, "kube-system", "pod-checkpointer", true, true); err != nil {
		t.Fatalf("verifyCheckpoint: %s", err)
	}
	if err := verifyCheckpoint(c, testNS, "nginx-daemonset", true, false); err != nil {
		t.Fatalf("verifyCheckpoint: %s", err)
	}

	// Disable the kubelet and reboot the worker.
	stdout, stderr, err := c.Workers[0].SSH("sudo systemctl disable kubelet")
	if err != nil {
		t.Fatalf("Failed to disable kubelet on worker %q: %v\nstdout: %s\nstderr: %s", c.Workers[0].Name, err, stdout, stderr)
	}
	if err := c.Workers[0].Reboot(); err != nil {
		t.Fatalf("Failed to reboot worker: %v", err)
	}

	// Delete test pod on the workers.
	patch = `{"spec":{"template":{"spec":{"nodeSelector":{"node-role.kubernetes.io/master":""}}}}}`
	_, err = client.ExtensionsV1beta1().DaemonSets(testNS).Patch("nginx-daemonset", types.MergePatchType, []byte(patch))
	if err != nil {
		t.Fatalf("unable to patch daemonset: %v", err)
	}

	// Disable the kubelet and reboot the masters.
	var rebootGroup sync.WaitGroup
	for i := range c.Masters {
		rebootGroup.Add(1)
		go func(i int) {
			defer rebootGroup.Done()
			stdout, stderr, err = c.Masters[i].SSH("sudo systemctl disable kubelet")
			if err != nil {
				t.Fatalf("Failed to disable kubelet on master %q: %v\nstdout: %s\nstderr: %s", c.Masters[i].Name, err, stdout, stderr)
			}
			if err := c.Masters[i].Reboot(); err != nil {
				t.Fatalf("Failed to reboot master: %v", err)
			}
		}(i)
	}
	rebootGroup.Wait()

	// Start the worker kubelet.
	stdout, stderr, err = c.Workers[0].SSH("sudo systemctl enable kubelet && sudo systemctl start kubelet")
	if err != nil {
		t.Fatalf("unable to start kubelet on worker %q: %v\nstdout: %s\nstderr: %s", c.Workers[0].Name, err, stdout, stderr)
	}

	// Verify that the checkpoints are running.
	if err := verifyPod(c, "pod-checkpointer", true); err != nil {
		t.Fatalf("verifyPod: %s", err)
	}
	if err := verifyPod(c, "nginx-daemonset", true); err != nil {
		t.Fatalf("verifyPod: %s", err)
	}

	// Start the master kubelets.
	var enableGroup sync.WaitGroup
	for i := range c.Masters {
		enableGroup.Add(1)
		go func(i int) {
			defer enableGroup.Done()
			stdout, stderr, err = c.Masters[i].SSH("sudo systemctl enable kubelet && sudo systemctl start kubelet")
			if err != nil {
				t.Fatalf("unable to start kubelet on master %q: %v\nstdout: %s\nstderr: %s", c.Masters[i].Name, err, stdout, stderr)
			}
		}(i)
	}
	enableGroup.Wait()

	// Verify that checkpoint is cleaned up and not running, but the pod checkpointer should still be running.
	if err := verifyPod(c, "pod-checkpointer", true); err != nil {
		t.Fatalf("verifyPod: %s", err)
	}
	if err := verifyPod(c, "nginx-daemonset", false); err != nil {
		t.Fatalf("verifyPod: %s", err)
	}
	if err := verifyCheckpoint(c, "kube-system", "pod-checkpointer", true, true); err != nil {
		t.Fatalf("verifyCheckpoint: %s", err)
	}
	if err := verifyCheckpoint(c, testNS, "nginx-daemonset", false, false); err != nil {
		t.Fatalf("verifyCheckpoint: %s", err)
	}
}
