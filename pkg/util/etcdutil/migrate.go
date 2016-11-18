package etcdutil

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/golang/glog"
	"golang.org/x/net/context"
	"k8s.io/kubernetes/pkg/api"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/release_1_4"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util/wait"
)

const (
	apiserverAddr = "http://127.0.0.1:8080"
	etcdServiceIP = "10.3.0.15"
)

func Migrate() error {
	kubecli, err := clientset.NewForConfig(&restclient.Config{
		Host: apiserverAddr,
	})
	if err != nil {
		return fmt.Errorf("fail to create kube client: %v", err)
	}
	httpcli := kubecli.CoreClient.RESTClient.Client

	err = waitEtcdTPRReady(httpcli, 5*time.Second, 60*time.Second, apiserverAddr, api.NamespaceSystem)
	if err != nil {
		return err
	}

	ip, err := getBootEtcdPodIP(kubecli)
	if err != nil {
		return err
	}
	glog.Infof("boot-etcd pod IP is: %s", ip)

	if err := createMigratedEtcdCluster(httpcli, apiserverAddr, ip); err != nil {
		glog.Errorf("fail to create migrated etcd cluster: %v", err)
		return err
	}

	return checkEtcdClusterUp()
}

func listETCDCluster(host, ns string, httpClient *http.Client) (*http.Response, error) {
	return httpClient.Get(fmt.Sprintf("%s/apis/coreos.com/v1/namespaces/%s/etcdclusters",
		host, ns))
}

func waitEtcdTPRReady(httpClient *http.Client, interval, timeout time.Duration, host, ns string) error {
	err := wait.Poll(interval, timeout, func() (bool, error) {
		resp, err := listETCDCluster(host, ns, httpClient)
		if err != nil {
			return false, err
		}
		defer resp.Body.Close()

		switch resp.StatusCode {
		case http.StatusOK:
			return true, nil
		case http.StatusNotFound: // not set up yet. wait.
			return false, nil
		default:
			return false, fmt.Errorf("invalid status code: %v", resp.Status)
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
		podList, err := kubecli.Core().Pods(api.NamespaceSystem).List(api.ListOptions{
			LabelSelector: labels.SelectorFromSet(labels.Set{"k8s-app": "boot-etcd"}),
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

func createMigratedEtcdCluster(httpcli *http.Client, host, podIP string) error {
	b := []byte(fmt.Sprintf(`{
  "apiVersion": "coreos.com/v1",
  "kind": "EtcdCluster",
  "metadata": {
    "name": "etcd-cluster",
    "namespace": "kube-system"
  },
  "spec": {
    "size": 1,
    "version": "v3.1.0-alpha.1",
    "selfHosted": {
		"bootMemberClientEndpoint": "http://%s:12379"
    }
  }
}`, podIP))

	resp, err := httpcli.Post(
		fmt.Sprintf("%s/apis/coreos.com/v1/namespaces/kube-system/etcdclusters", host),
		"application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("fail to create etcd cluster object, status (%v), object (%s)", resp.Status, string(b))
	}
	return nil
}

func checkEtcdClusterUp() error {
	glog.Infof("initial delaying (30s)...")
	time.Sleep(30 * time.Second)

	// The checking does:
	// - Trying to talk to etcd cluster via etcd service. The assumption here is that
	//   the etcd service only selects the etcd pods newly created, excluding the boot one.
	//   Once we can talk to it, we are sure that etcd cluster is created.
	// - Then we list members and see if it's been reduced to 1 member. Because when we
	//   can talk to the etcd cluster, we are certain there are 2 members at the beginning,
	//   and will reduce to 1 eventually. That's the timeline of expected events.
	//   As long as 1 member cluster is reached, we are certain cluster has been migrated successfully.
	err := wait.PollImmediate(10*time.Second, 60*time.Second, func() (bool, error) {
		cfg := clientv3.Config{
			Endpoints:   []string{fmt.Sprintf("http://%s:2379", etcdServiceIP)},
			DialTimeout: 5 * time.Second,
		}
		etcdcli, err := clientv3.New(cfg)
		if err != nil {
			glog.Errorf("fail to create etcd client, retrying...: %v", err)
			return false, nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		m, err := etcdcli.MemberList(ctx)
		if err != nil {
			glog.Errorf("fail to list etcd members, retrying...: %v", err)
			return false, nil
		}
		if len(m.Members) != 1 {
			glog.Infof("Still migrating boot etcd member, retrying...")
			return false, nil
		}
		glog.Infof("etcd cluster is up. Member: %v", m.Members[0].Name)
		return true, nil
	})
	return err
}
