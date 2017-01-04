package etcdutil

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/coreos/etcd-operator/pkg/spec"
	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/release_1_5"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/util/wait"
)

const (
	apiserverAddr = "http://127.0.0.1:8080"
	etcdServiceIP = "10.3.0.15"

	etcdClusterName = "kube-etcd"
)

var waitEtcdClusterRunningTime = 300 * time.Second

func Migrate() error {
	kubecli, err := clientset.NewForConfig(&restclient.Config{
		Host: apiserverAddr,
	})
	if err != nil {
		return fmt.Errorf("fail to create kube client: %v", err)
	}
	restClient := kubecli.CoreV1().RESTClient()

	err = waitEtcdTPRReady(restClient, 5*time.Second, 60*time.Second, api.NamespaceSystem)
	if err != nil {
		return err
	}

	ip, err := getBootEtcdPodIP(kubecli)
	if err != nil {
		return err
	}
	glog.Infof("boot-etcd pod IP is: %s", ip)

	if err := createMigratedEtcdCluster(restClient, apiserverAddr, ip); err != nil {
		glog.Errorf("fail to create migrated etcd cluster: %v", err)
		return err
	}

	return waitEtcdClusterRunning(restClient, apiserverAddr)
}

func listETCDCluster(ns string, restClient restclient.Interface) restclient.Result {
	uri := fmt.Sprintf("/apis/coreos.com/v1/namespaces/%s/etcdclusters", ns)
	return restClient.Get().RequestURI(uri).Do()
}

func waitEtcdTPRReady(restClient restclient.Interface, interval, timeout time.Duration, ns string) error {
	err := wait.Poll(interval, timeout, func() (bool, error) {
		res := listETCDCluster(ns, restClient)
		if res.Error() != nil {
			return false, res.Error()
		}

		var status int
		res.StatusCode(&status)

		switch status {
		case http.StatusOK:
			return true, nil
		case http.StatusNotFound: // not set up yet. wait.
			return false, nil
		default:
			return false, fmt.Errorf("invalid status code: %v", status)
		}
	})
	if err != nil {
		return fmt.Errorf("fail to wait etcd TPR to be ready: %v", err)
	}
	return nil
}

func getBootEtcdPodIP(kubecli clientset.Interface) (string, error) {
	var ip string
	err := wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
		podList, err := kubecli.CoreV1().Pods(api.NamespaceSystem).List(v1.ListOptions{
			LabelSelector: "k8s-app=boot-etcd",
		})
		if err != nil {
			glog.Errorf("fail to list 'boot-etcd' pod: %v", err)
			return false, err
		}
		if len(podList.Items) < 1 {
			glog.Warningf("no 'boot-etcd' pod found, retrying after 5s...")
			return false, nil
		}

		pod := podList.Items[0]
		ip = pod.Status.PodIP
		if len(ip) == 0 {
			return false, nil
		}
		return true, nil
	})
	return ip, err
}

func createMigratedEtcdCluster(restclient restclient.Interface, host, podIP string) error {
	b := []byte(fmt.Sprintf(`{
  "apiVersion": "coreos.com/v1",
  "kind": "EtcdCluster",
  "metadata": {
    "name": "%s",
    "namespace": "kube-system"
  },
  "spec": {
    "size": 1,
    "version": "v3.1.0-alpha.1",
    "selfHosted": {
		"bootMemberClientEndpoint": "http://%s:12379"
    }
  }
}`, etcdClusterName, podIP))

	uri := "/apis/coreos.com/v1/namespaces/kube-system/etcdclusters"
	res := restclient.Post().RequestURI(uri).SetHeader("Content-Type", "application/json").Body(b).Do()

	if res.Error() != nil {
		return res.Error()
	}

	var status int
	res.StatusCode(&status)

	if status != http.StatusCreated {
		return fmt.Errorf("fail to create etcd cluster object, status (%v), object (%s)", status, string(b))
	}
	return nil
}

func waitEtcdClusterRunning(restclient restclient.Interface, host string) error {
	glog.Infof("initial delaying (30s)...")
	time.Sleep(30 * time.Second)

	err := wait.Poll(10*time.Second, waitEtcdClusterRunningTime, func() (bool, error) {
		res := restclient.Get().RequestURI(makeEtcdClusterURI(host, etcdClusterName)).Do()
		if res.Error() != nil {
			return false, res.Error()
		}

		var status int
		res.StatusCode(&status)

		if status != http.StatusOK {
			return false, fmt.Errorf("invalid status code: %v", status)
		}

		e := &spec.EtcdCluster{}
		err := res.Into(e)
		if err != nil {
			return false, err
		}
		switch e.Status.Phase {
		case spec.ClusterPhaseRunning:
			return true, nil
		case spec.ClusterPhaseFailed:
			return false, errors.New("failed to create etcd cluster")
		default:
			// All the other phases are not ready
			return false, nil
		}
	})
	return fmt.Errorf("wait etcd cluster running failed: %v", err)
}

func makeEtcdClusterURI(host, name string) string {
	return fmt.Sprintf("%s/apis/coreos.com/v1/namespaces/kube-system/etcdclusters/%s", host, name)
}
