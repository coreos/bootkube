package etcdutil

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/coreos/etcd-operator/pkg/spec"
	"github.com/coreos/etcd/clientv3"
	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	apierrors "k8s.io/kubernetes/pkg/api/errors"
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

var (
	waitEtcdClusterRunningTime = 300 * time.Second
	waitBootEtcdRemovedTime    = 300 * time.Second
)

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
	glog.Infof("etcd cluster TPR is setup")

	ip, err := getBootEtcdPodIP(kubecli)
	if err != nil {
		return err
	}
	glog.Infof("boot-etcd pod IP is: %s", ip)

	if err := createMigratedEtcdCluster(restClient, apiserverAddr, ip); err != nil {
		return fmt.Errorf("fail to create migrated etcd cluster: %v", err)
	}
	glog.Infof("etcd cluster for migration is created")

	if err := waitEtcdClusterRunning(restClient); err != nil {
		return fmt.Errorf("wait etcd cluster running failed: %v", err)
	}
	glog.Info("etcd cluster for migration is now running")

	if err := waitBootEtcdRemoved(); err != nil {
		return fmt.Errorf("wait boot etcd deleted failed: %v", err)
	}
	glog.Info("the boot etcd is removed from the migration cluster")
	return os.Remove(bootEtcdFilePath)
}

func listETCDCluster(ns string, restClient restclient.Interface) restclient.Result {
	uri := fmt.Sprintf("/apis/coreos.com/v1/namespaces/%s/etcdclusters", ns)
	return restClient.Get().RequestURI(uri).Do()
}

func waitEtcdTPRReady(restClient restclient.Interface, interval, timeout time.Duration, ns string) error {
	err := wait.Poll(interval, timeout, func() (bool, error) {
		res := listETCDCluster(ns, restClient)
		if err := res.Error(); err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
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
    "version": "v3.1.0",
    "selfHosted": {
		"bootMemberClientEndpoint": "http://%s:12379"
    }
  }
}`, etcdClusterName, podIP))

	uri := "/apis/coreos.com/v1/namespaces/kube-system/etcdclusters"
	res := restclient.Post().RequestURI(uri).SetHeader("Content-Type", "application/json").Body(b).Do()

	return res.Error()
}

func waitEtcdClusterRunning(restclient restclient.Interface) error {
	glog.Infof("initial delaying (30s)...")
	time.Sleep(30 * time.Second)

	err := wait.Poll(10*time.Second, waitEtcdClusterRunningTime, func() (bool, error) {
		b, err := restclient.Get().RequestURI(makeEtcdClusterURI(etcdClusterName)).DoRaw()
		if err != nil {
			return false, fmt.Errorf("fail to get etcdcluster: %v", err)
		}

		e := &spec.EtcdCluster{}
		if err := json.Unmarshal(b, e); err != nil {
			return false, err
		}
		if e.Status == nil {
			return false, nil
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
	return err
}

func waitBootEtcdRemoved() error {
	err := wait.Poll(10*time.Second, waitBootEtcdRemovedTime, func() (bool, error) {
		cfg := clientv3.Config{
			Endpoints:   []string{fmt.Sprintf("http://%s:2379", etcdServiceIP)},
			DialTimeout: 5 * time.Second,
		}
		etcdcli, err := clientv3.New(cfg)
		if err != nil {
			glog.Errorf("fail to create etcd client, will retry: %v", err)
			return false, nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		m, err := etcdcli.MemberList(ctx)
		cancel()
		etcdcli.Close()
		if err != nil {
			glog.Errorf("fail to list member, will retry: %v", err)
			return false, nil
		}

		if len(m.Members) != 1 {
			glog.Info("Still waiting boot etcd to be deletd...")
			return false, nil
		}
		return true, nil
	})
	return err
}

func makeEtcdClusterURI(name string) string {
	return fmt.Sprintf("/apis/coreos.com/v1/namespaces/kube-system/etcdclusters/%s", name)
}
