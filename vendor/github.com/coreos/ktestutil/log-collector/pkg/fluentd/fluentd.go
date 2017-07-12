package fluentd

import (
	"fmt"
	"time"

	"github.com/coreos/ktestutil/utils"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
)

const (
	fluentMasterNodeAnnotation = "log-collector.github.com/fluentd-master"
)

// CreateAssets creates fluentd master deployment and fluentd worker daemonset.
// all fluentd workers stream their local logs to the fluentd master.
// fluentd master writes all the logs to the disk at `/var/log/log-collector/`
// fluentd master is pinned to a master node with annotation `log-collector.github.com/fluentd-master`.
//
// CreateAssets waits for fluentd master pod and at least one fluentd worker pod to be running.
func CreateAssets(client kubernetes.Interface, namespace string) error {
	// select and tag one of the masters
	if err := wait.PollImmediate(time.Second, 5*time.Minute, func() (bool, error) {
		nl, err := client.CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/master="})
		if err != nil || len(nl.Items) == 0 {
			return false, fmt.Errorf("couldn't find master node to run fluentd master %v", err)
		}
		n := nl.Items[0]
		n.ObjectMeta.Labels[fluentMasterNodeAnnotation] = ""
		if _, err := client.CoreV1().Nodes().Update(&n); err != nil {
			if errors.IsConflict(err) {
				return false, nil
			}
			return false, fmt.Errorf("couldn't tag one of the master nodes %v", err)
		}
		return true, nil
	}); err != nil {
		return err
	}

	// create assests
	if err := createMasterCfg(client, namespace); err != nil {
		return fmt.Errorf("error creating fluentd asset %v", err)
	}
	if err := createMasterDeploy(client, namespace); err != nil {
		return fmt.Errorf("error creating fluentd asset %v", err)
	}
	if err := createMasterSvc(client, namespace); err != nil {
		return fmt.Errorf("error creating fluentd asset %v", err)
	}

	if err := createWorkerCfg(client, namespace); err != nil {
		return fmt.Errorf("error creating fluentd asset %v", err)
	}
	if err := createWorkerDs(client, namespace); err != nil {
		return fmt.Errorf("error creating fluentd asset %v", err)
	}

	if err := utils.Retry(10, time.Second*10, waitPodReadyOnMaster(client, namespace)); err != nil {
		return err
	}
	if err := utils.Retry(10, time.Second*10, waitPodReadyOnWorker(client, namespace)); err != nil {
		return err
	}

	return nil
}

// DeleteAssets deletes all the fluentd assets.
// Also removes all node annotations.
func DeleteAssets(client kubernetes.Interface, namespace string) error {
	if err := deleteMasterCfg(client, namespace); err != nil {
		return fmt.Errorf("error deleting fluentd asset %v", err)
	}
	if err := deleteMasterDeploy(client, namespace); err != nil {
		return fmt.Errorf("error deleting fluentd asset %v", err)
	}
	if err := deleteMasterSvc(client, namespace); err != nil {
		return fmt.Errorf("error deleting fluentd asset %v", err)
	}

	if err := deleteWorkerCfg(client, namespace); err != nil {
		return fmt.Errorf("error deleting fluentd asset %v", err)
	}
	if err := deleteWorkerDs(client, namespace); err != nil {
		return fmt.Errorf("error deleting fluentd asset %v", err)
	}

	if err := wait.PollImmediate(time.Second, 5*time.Minute, func() (bool, error) {
		nl, err := client.CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=", fluentMasterNodeAnnotation)})
		if err != nil || len(nl.Items) == 0 {
			return false, fmt.Errorf("couldn't find master node running fluentd master %v", err)
		}
		n := nl.Items[0]
		delete(n.ObjectMeta.Labels, fluentMasterNodeAnnotation)
		if _, err := client.CoreV1().Nodes().Update(&n); err != nil {
			if errors.IsConflict(err) {
				return false, nil
			}
			return false, fmt.Errorf("couldn't delete annotation from the master node running fluentd master %v", err)
		}
		return true, nil
	}); err != nil {
		return err
	}

	return nil
}

// GetNodeAddressWithMaster returns PublicAddressableIP of the master node that is running fluent master.
func GetNodeAddressWithMaster(client kubernetes.Interface, namespace string) (string, error) {
	pl, err := client.CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: "k8s-app=fluentd-master,tier=control-plane"})
	if err != nil || len(pl.Items) == 0 {
		return "", fmt.Errorf("No pod from fluentd-master deployment found: %v", err)
	}

	p := &pl.Items[0]
	nodeName := p.Spec.NodeName

	n, err := client.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("No node found with name: %s", nodeName)
	}

	var host string
	for _, addr := range n.Status.Addresses {
		if addr.Type == v1.NodeExternalIP {
			host = addr.Address
		}

	}
	if host == "" {
		// try finding "LegacyHostIP"
		for _, addr := range n.Status.Addresses {
			if addr.Type == "LegacyHostIP" {
				host = addr.Address
			}
		}
	}

	if host == "" {
		return "", fmt.Errorf("could not get external node IP for node: %s", nodeName)
	}

	return host, nil
}

func waitPodReadyOnMaster(client kubernetes.Interface, namespace string) func() error {
	return func() error {
		l, err := client.CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: "k8s-app=fluentd-master"})
		if err != nil || len(l.Items) == 0 {
			return fmt.Errorf("pod not yet running: %v", err)
		}

		// take the first pod
		p := &l.Items[0]

		if p.Status.Phase != v1.PodRunning {
			return fmt.Errorf("pod not yet running: %v", p.Status.Phase)
		}
		return nil
	}
}

func waitPodReadyOnWorker(client kubernetes.Interface, namespace string) func() error {
	return func() error {
		l, err := client.CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: "k8s-app=fluentd-worker"})
		if err != nil || len(l.Items) == 0 {
			return fmt.Errorf("pod not yet running: %v", err)
		}

		// take the first pod
		p := &l.Items[0]

		if p.Status.Phase != v1.PodRunning {
			return fmt.Errorf("pod not yet running: %v", p.Status.Phase)
		}
		return nil
	}
}
