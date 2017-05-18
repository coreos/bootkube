package etcdutil

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kubernetes-incubator/bootkube/pkg/asset"

	"github.com/coreos/etcd-operator/pkg/spec"
	"github.com/coreos/etcd/clientv3"
	"github.com/golang/glog"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	etcdClusterName = "kube-etcd"
)

var (
	waitEtcdClusterRunningTime = 300 * time.Second
	waitBootEtcdRemovedTime    = 300 * time.Second
)

func Migrate(kubeConfig clientcmd.ClientConfig) error {
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to create kube client config: %v", err)
	}
	kubecli, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kube client: %v", err)
	}
	restClient := kubecli.CoreV1().RESTClient()

	err = waitEtcdTPRReady(restClient, 5*time.Second, 60*time.Second, api.NamespaceSystem)
	if err != nil {
		return err
	}
	glog.Infof("created etcd cluster TPR")

	ip, err := getBootEtcdPodIP(kubecli)
	if err != nil {
		return err
	}
	etcdServiceIP, err := getServiceIP(kubecli, api.NamespaceSystem, asset.EtcdServiceName)
	if err != nil {
		return err
	}
	glog.Infof("boot-etcd pod IP is: %s, etcd-service IP is %s", ip, etcdServiceIP)

	if err := createMigratedEtcdCluster(restClient, ip); err != nil {
		return fmt.Errorf("failed to create etcd cluster for migration: %v", err)
	}
	glog.Infof("created etcd cluster for migration")

	if err := waitEtcdClusterRunning(restClient); err != nil {
		return fmt.Errorf("failed to wait for etcd cluster's status to be running: %v", err)
	}
	glog.Info("etcd cluster for migration is now running")

	if err := waitBootEtcdRemoved(etcdServiceIP); err != nil {
		return fmt.Errorf("failed to wait for boot-etcd to be removed: %v", err)
	}
	glog.Info("removed boot-etcd from the etcd cluster")
	return nil
}

func listETCDCluster(ns string, restClient restclient.Interface) restclient.Result {
	uri := fmt.Sprintf("/apis/%s/%s/namespaces/%s/%s", spec.TPRGroup, spec.TPRVersion, ns, spec.TPRKindPlural)
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
		return fmt.Errorf("failed to wait for etcd TPR to be ready: %v", err)
	}
	return nil
}

func getBootEtcdPodIP(kubecli kubernetes.Interface) (string, error) {
	var ip string
	err := wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
		podList, err := kubecli.CoreV1().Pods(api.NamespaceSystem).List(v1.ListOptions{
			LabelSelector: "k8s-app=boot-etcd",
		})
		if err != nil {
			glog.Errorf("failed to list 'boot-etcd' pod: %v", err)
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

func createMigratedEtcdCluster(restclient restclient.Interface, podIP string) error {
	b := []byte(fmt.Sprintf(`{
  "apiVersion": "%s/%s",
  "kind": "%s",
  "metadata": {
    "name": "%s",
    "namespace": "kube-system"
  },
  "spec": {
    "size": 1,
    "version": "v3.1.6",
    "pod": {
      "nodeSelector": {
        "node-role.kubernetes.io/master": ""
      },
      "antiAffinity": true,
      "tolerations": [
        {
          "key": "node-role.kubernetes.io/master",
          "operator": "Exists",
          "effect": "NoSchedule"
        }
      ]
    },
    "selfHosted": {
      "bootMemberClientEndpoint": "http://%s:12379"
    }
  }
}`, spec.TPRGroup, spec.TPRVersion, strings.Title(spec.TPRKind), etcdClusterName, podIP))

	uri := fmt.Sprintf("/apis/%s/%s/namespaces/kube-system/%s", spec.TPRGroup, spec.TPRVersion, spec.TPRKindPlural)
	res := restclient.Post().RequestURI(uri).SetHeader("Content-Type", "application/json").Body(b).Do()

	return res.Error()
}

func waitEtcdClusterRunning(restclient restclient.Interface) error {
	glog.Infof("initial delaying (30s)...")
	time.Sleep(30 * time.Second)

	err := wait.Poll(10*time.Second, waitEtcdClusterRunningTime, func() (bool, error) {
		b, err := restclient.Get().RequestURI(makeEtcdClusterURI(etcdClusterName)).DoRaw()
		if err != nil {
			return false, fmt.Errorf("failed to get etcd cluster TPR: %v", err)
		}

		e := &spec.Cluster{}
		if err := json.Unmarshal(b, e); err != nil {
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
	return err
}

func getServiceIP(kubecli kubernetes.Interface, ns, svcName string) (string, error) {
	svc, err := kubecli.CoreV1().Services(ns).Get(svcName, v1.GetOptions{})
	if err != nil {
		return "", err
	}
	return svc.Spec.ClusterIP, nil
}

func waitBootEtcdRemoved(etcdServiceIP string) error {
	err := wait.Poll(10*time.Second, waitBootEtcdRemovedTime, func() (bool, error) {
		cfg := clientv3.Config{
			Endpoints:   []string{fmt.Sprintf("http://%s:2379", etcdServiceIP)},
			DialTimeout: 5 * time.Second,
		}
		etcdcli, err := clientv3.New(cfg)
		if err != nil {
			glog.Errorf("failed to create etcd client, will retry: %v", err)
			return false, nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		m, err := etcdcli.MemberList(ctx)
		cancel()
		etcdcli.Close()
		if err != nil {
			glog.Errorf("failed to list etcd members, will retry: %v", err)
			return false, nil
		}

		if len(m.Members) != 1 {
			glog.Info("still waiting for boot-etcd to be deleted...")
			return false, nil
		}
		return true, nil
	})
	return err
}

func makeEtcdClusterURI(name string) string {
	return fmt.Sprintf("/apis/%s/%s/namespaces/kube-system/%s/%s", spec.TPRGroup, spec.TPRVersion, spec.TPRKindPlural, name)
}
