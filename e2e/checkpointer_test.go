package e2e

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	rbac "k8s.io/api/rbac/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/kubernetes-incubator/bootkube/pkg/asset"
)

const (
	retryAttempts = 60
	retryInterval = 5 * time.Second
)

func TestCheckpointer(t *testing.T) {
	t.Run("UnscheduleCheckpointer", testCheckpointerUnscheduleCheckpointer)
	t.Run("UnscheduleParent", testCheckpointerUnscheduleParent)
}

// Deleting a checkpointer should cleanup any other checkpointed manifests on that node.
func testCheckpointerUnscheduleCheckpointer(t *testing.T) {
	// Get the cluster
	c := waitCluster(t)

	testNS := makeNamespace(t.Name())
	if _, err := createNamespace(client, testNS); err != nil {
		t.Fatalf("Failed to create namespace: %v", err)
	}
	defer deleteNamespace(client, testNS)

	// Deploy the pod checkpointer and test nginx.
	if err := setupTestCheckpointerRole(testNS); err != nil {
		t.Fatalf("Failed to create test-checkpointer role: %v", err)
	}
	if err := createDaemonSet(testNS, []byte(fmt.Sprintf(checkpointerDS, asset.DefaultImages.PodCheckpointer)), c); err != nil {
		t.Fatalf("Failed to create pod-checkpointer daemonset: %v", err)
	}
	if err := createDaemonSet(testNS, nginxDS, c); err != nil {
		t.Fatalf("Failed to create nginx daemonset: %v", err)
	}

	if err := retry(10, 1*time.Second, func() error {
		// Verify the checkpoints are created.
		if err := verifyCheckpoint(c, testNS, "test-checkpointer", true, true); err != nil {
			err = fmt.Errorf("Failed to verify checkpoint: %v", err)
			t.Log(err)
			return err
		}
		if err := verifyCheckpoint(c, testNS, "test-nginx", true, false); err != nil {
			err = fmt.Errorf("Failed to verify checkpoint: %v", err)
			t.Log(err)
			return err
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// Delete the pod checkpointer.
	deletePropagationForeground := metav1.DeletePropagationForeground
	if err := client.ExtensionsV1beta1().DaemonSets(testNS).Delete("test-checkpointer", &metav1.DeleteOptions{PropagationPolicy: &deletePropagationForeground}); err != nil {
		t.Fatalf("Failed to delete checkpointer: %v", err)
	}

	// Verify that the test-checkpointer is cleaned up but the nginx daemonset is still running.
	if err := verifyPod(c, "test-checkpointer", false); err != nil {
		t.Errorf("Failed to verifyPod: %s", err)
	}
	if err := verifyPod(c, "test-nginx", true); err != nil {
		t.Errorf("Failed to verifyPod: %s", err)
	}

	// Currently disabled due to e2e testing flakes. See:
	// * https://github.com/kubernetes-incubator/bootkube/issues/816
	// * https://github.com/kubernetes-incubator/bootkube/pull/836
	// if err := verifyCheckpoint(c, testNS, "test-checkpointer", false, false); err != nil {
	//   t.Fatalf("Failed to verifyCheckpoint: %s", err)
	// }

	// Checkpointed manifest should be cleaned up.
	if err := verifyCheckpoint(c, testNS, "test-nginx", false, false); err != nil {
		t.Errorf("Failed to verifyCheckpoint: %s", err)
	}
}

// 1. Schedule a pod checkpointer on worker node.
// 2. Schedule a test pod on worker node.
// 3. Reboot the worker without starting the kubelet.
// 4. Delete the test pod on API server.
// 5. Reboot the masters without starting the kubelet.
// 6. Start the worker kubelet, verify the checkpointer and the pod are still running as static pods.
// 7. Start the master kubelets, verify the test pod is removed, but not the checkpointer.
func testCheckpointerUnscheduleParent(t *testing.T) {
	// Get the cluster
	c := waitCluster(t)

	testNS := makeNamespace(t.Name())
	if _, err := createNamespace(client, testNS); err != nil {
		t.Fatalf("Failed to create namespace: %v", err)
	}
	defer deleteNamespace(client, testNS)

	// Deploy the pod checkpointer and test nginx.
	if err := setupTestCheckpointerRole(testNS); err != nil {
		t.Fatalf("Failed to create test-checkpointer role: %v", err)
	}
	if err := createDaemonSet(testNS, []byte(fmt.Sprintf(checkpointerDS, asset.DefaultImages.PodCheckpointer)), c); err != nil {
		t.Fatalf("Failed to create pod-checkpointer daemonset: %v", err)
	}
	if err := createDaemonSet(testNS, nginxDS, c); err != nil {
		t.Fatalf("Failed to create nginx daemonset: %v", err)
	}

	// Verify the checkpoints are created.
	if err := verifyCheckpoint(c, testNS, "test-checkpointer", true, true); err != nil {
		t.Fatalf("verifyCheckpoint: %s", err)
	}
	if err := verifyCheckpoint(c, testNS, "test-nginx", true, false); err != nil {
		t.Fatalf("verifyCheckpoint: %s", err)
	}

	// Disable the kubelet and reboot the worker.
	if stdout, stderr, err := c.Workers[0].SSH("sudo systemctl disable kubelet"); err != nil {
		t.Fatalf("Failed to disable kubelet on worker %q: %v\nstdout: %s\nstderr: %s", c.Workers[0].Name, err, stdout, stderr)
	}
	if err := c.Workers[0].Reboot(); err != nil {
		t.Fatalf("Failed to reboot worker: %v", err)
	}

	// Delete test pod on the workers.
	patch := `{"spec":{"template":{"spec":{"nodeSelector":{"node-role.kubernetes.io/master":""}}}}}`
	if _, err := client.ExtensionsV1beta1().DaemonSets(testNS).Patch("test-nginx", types.MergePatchType, []byte(patch)); err != nil {
		t.Fatalf("unable to patch daemonset: %v", err)
	}

	// Disable the kubelet and reboot the masters.
	var rebootGroup sync.WaitGroup
	for i := range c.Masters {
		rebootGroup.Add(1)
		go func(i int) {
			defer rebootGroup.Done()
			if stdout, stderr, err := c.Masters[i].SSH("sudo systemctl disable kubelet"); err != nil {
				t.Fatalf("Failed to disable kubelet on master %q: %v\nstdout: %s\nstderr: %s", c.Masters[i].Name, err, stdout, stderr)
			}
			if err := c.Masters[i].Reboot(); err != nil {
				t.Fatalf("Failed to reboot master: %v", err)
			}
		}(i)
	}
	rebootGroup.Wait()

	// Start the worker kubelet.
	if stdout, stderr, err := c.Workers[0].SSH("sudo systemctl enable kubelet && sudo systemctl start kubelet"); err != nil {
		t.Fatalf("unable to start kubelet on worker %q: %v\nstdout: %s\nstderr: %s", c.Workers[0].Name, err, stdout, stderr)
	}

	// Verify that the checkpoints are running.
	if err := verifyPod(c, "pod-checkpointer", true); err != nil {
		t.Fatalf("verifyPod: %s", err)
	}
	if err := verifyPod(c, "test-nginx", true); err != nil {
		t.Fatalf("verifyPod: %s", err)
	}

	// Start the master kubelets.
	var enableGroup sync.WaitGroup
	for i := range c.Masters {
		enableGroup.Add(1)
		go func(i int) {
			defer enableGroup.Done()
			if stdout, stderr, err := c.Masters[i].SSH("sudo systemctl enable kubelet && sudo systemctl start kubelet"); err != nil {
				t.Fatalf("unable to start kubelet on master %q: %v\nstdout: %s\nstderr: %s", c.Masters[i].Name, err, stdout, stderr)
			}
		}(i)
	}
	enableGroup.Wait()

	// Verify that checkpoint is cleaned up and not running, but the pod checkpointer should still be running.
	if err := verifyPod(c, "test-checkpointer", true); err != nil {
		t.Fatalf("verifyPod: %s", err)
	}
	if err := verifyPod(c, "test-nginx", false); err != nil {
		t.Fatalf("verifyPod: %s", err)
	}
	if err := verifyCheckpoint(c, testNS, "test-checkpointer", true, true); err != nil {
		t.Fatalf("verifyCheckpoint: %s", err)
	}
	if err := verifyCheckpoint(c, testNS, "test-nginx", false, false); err != nil {
		t.Fatalf("verifyCheckpoint: %s", err)
	}
}

func makeNamespace(testName string) string {
	return strings.ToLower(fmt.Sprintf("%s-%s", namespace, strings.Split(testName, "/")[1]))
}

func createDaemonSet(namespace string, manifest []byte, c *Cluster) error {
	obj, _, err := scheme.Codecs.UniversalDecoder().Decode(manifest, nil, &v1beta1.DaemonSet{})
	if err != nil {
		return fmt.Errorf("unable to decode manifest: %v", err)
	}
	ds, ok := obj.(*v1beta1.DaemonSet)
	if !ok {
		return fmt.Errorf("expected manifest to decode into *api.Daemonset, got %T", ds)
	}
	if _, err := client.ExtensionsV1beta1().DaemonSets(namespace).Create(ds); err != nil {
		return fmt.Errorf("failed to create the checkpoint parent: %v", err)
	}
	if err := verifyPod(c, ds.ObjectMeta.Name, true); err != nil {
		return fmt.Errorf("failed to verifyPod: %s", err)
	}
	return nil
}

func verifyCheckpoint(c *Cluster, namespace, daemonsetName string, shouldExist, shouldBeActive bool) error {
	return retry(retryAttempts, retryInterval, func() error {
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
	})
}

func verifyPod(c *Cluster, daemonsetName string, shouldRun bool) error {
	return retry(retryAttempts, retryInterval, func() error {
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
	})
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

	if err := retry(retryAttempts, retryInterval, func() error {
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
	}); err != nil {
		t.Fatalf("Failed to wait cluster: %v", err)
	}
	return c
}

func setupTestCheckpointerRole(namespace string) error {
	// Copy special kubeconfig-in-cluster configmap from kube-system namespace.
	kc, err := client.CoreV1().ConfigMaps(metav1.NamespaceSystem).Get("kubeconfig-in-cluster", metav1.GetOptions{})
	if err != nil {
		return err
	}
	kc.ObjectMeta = metav1.ObjectMeta{
		Name:      kc.ObjectMeta.Name,
		Namespace: namespace,
	}
	if _, err := client.CoreV1().ConfigMaps(namespace).Create(kc); err != nil {
		return err
	}

	if _, err := client.CoreV1().ServiceAccounts(namespace).Create(&v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-checkpointer",
			Namespace: namespace,
		},
	}); err != nil {
		return err
	}
	if _, err := client.RbacV1beta1().Roles(namespace).Create(&rbac.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-checkpointer",
			Namespace: namespace,
		},
		Rules: []rbac.PolicyRule{{
			APIGroups: []string{""}, // "" indicates the core API group
			Resources: []string{"pods"},
			Verbs:     []string{"get", "watch", "list"},
		}, {
			APIGroups: []string{""}, // "" indicates the core API group
			Resources: []string{"secrets", "configmaps"},
			Verbs:     []string{"get"},
		}},
	}); err != nil {
		return err
	}
	if _, err := client.RbacV1beta1().RoleBindings(namespace).Create(&rbac.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-checkpointer",
			Namespace: namespace,
		},
		Subjects: []rbac.Subject{{
			Kind:      "ServiceAccount",
			Name:      "test-checkpointer",
			Namespace: namespace,
		}},
		RoleRef: rbac.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     "test-checkpointer",
		},
	}); err != nil {
		return err
	}
	return nil
}

const checkpointerDS = `apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: test-checkpointer
  labels:
    app: test-checkpointer
spec:
  selector:
    matchLabels:
      app: test-checkpointer
  template:
    metadata:
      labels:
        app: test-checkpointer
      annotations:
        checkpointer.alpha.coreos.com/checkpoint: "true"
    spec:
      containers:
      - name: test-checkpointer
        image: %s
        command:
        - /checkpoint
        - --checkpoint-grace-period=5s
        - --lock-file=/var/run/lock/test-checkpointer.lock
        - --kubeconfig=/etc/checkpointer/kubeconfig
        env:
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        imagePullPolicy: Always
        volumeMounts:
        - mountPath: /etc/checkpointer
          name: kubeconfig
        - mountPath: /etc/kubernetes
          name: etc-kubernetes
        - mountPath: /var/run
          name: var-run
      serviceAccountName: test-checkpointer
      hostNetwork: true
      restartPolicy: Always
      volumes:
      - name: kubeconfig
        configMap:
          name: kubeconfig-in-cluster
      - name: etc-kubernetes
        hostPath:
          path: /etc/kubernetes
      - name: var-run
        hostPath:
          path: /var/run
`

var nginxDS = []byte(`apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: test-nginx
spec:
  selector:
    matchLabels:
      app: nginx-checkpoint-test
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
          image: nginx
`)
