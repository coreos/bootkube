package etcdutil

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/coreos/etcd-operator/pkg/spec"
	"github.com/coreos/etcd/clientv3"
	"github.com/golang/glog"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

func Migrate(kubeConfig clientcmd.ClientConfig, svcPath, tprPath, etcdServiceIP string) error {
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

	if err := createBootstrapEtcdService(restClient, svcPath); err != nil {
		return fmt.Errorf("failed to create bootstrap-etcd-service: %v", err)
	}
	defer cleanupBootstrapEtcdService(restClient)

	if err := createMigratedEtcdCluster(restClient, tprPath); err != nil {
		return fmt.Errorf("failed to create etcd cluster for migration: %v", err)
	}
	glog.Infof("created etcd cluster for migration")

	if err := waitEtcdClusterRunning(restClient); err != nil {
		return err
	}
	glog.Info("etcd cluster for migration is now running")

	if err := waitBootEtcdRemoved(etcdServiceIP); err != nil {
		return fmt.Errorf("failed to wait for boot-etcd to be removed: %v", err)
	}
	glog.Info("removed boot-etcd from the etcd cluster")
	return nil
}

func listEtcdCluster(ns string, restClient restclient.Interface) restclient.Result {
	uri := fmt.Sprintf("/apis/%s/%s/namespaces/%s/%s", spec.TPRGroup, spec.TPRVersion, ns, spec.TPRKindPlural)
	return restClient.Get().RequestURI(uri).Do()
}

func waitEtcdTPRReady(restClient restclient.Interface, interval, timeout time.Duration, ns string) error {
	err := wait.Poll(interval, timeout, func() (bool, error) {
		res := listEtcdCluster(ns, restClient)
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

func createBootstrapEtcdService(restclient restclient.Interface, svcPath string) error {
	svc, err := ioutil.ReadFile(svcPath)
	if err != nil {
		return err
	}
	return restclient.Post().RequestURI("/api/v1/namespaces/kube-system/services").SetHeader("Content-Type", "application/json").Body(svc).Do().Error()
}

func createMigratedEtcdCluster(restclient restclient.Interface, tprPath string) error {
	tpr, err := ioutil.ReadFile(tprPath)
	if err != nil {
		return err
	}
	uri := fmt.Sprintf("/apis/%s/%s/namespaces/kube-system/%s", spec.TPRGroup, spec.TPRVersion, spec.TPRKindPlural)
	return restclient.Post().RequestURI(uri).SetHeader("Content-Type", "application/json").Body(tpr).Do().Error()
}

func waitEtcdClusterRunning(restclient restclient.Interface) error {
	glog.Infof("initial delaying (30s)...")
	time.Sleep(30 * time.Second)
	uri := fmt.Sprintf("/apis/%s/%s/namespaces/kube-system/%s/%s", spec.TPRGroup, spec.TPRVersion, spec.TPRKindPlural, etcdClusterName)
	err := wait.Poll(10*time.Second, waitEtcdClusterRunningTime, func() (bool, error) {
		b, err := restclient.Get().RequestURI(uri).DoRaw()
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

func cleanupBootstrapEtcdService(restclient restclient.Interface) {
	err := restclient.Delete().RequestURI("/api/v1/namespaces/kube-system/services/bootstrap-etcd-service").Do().Error()
	if err != nil {
		glog.Errorf("failed to remove bootstrap-etcd-service: %v", err)
	}
}
