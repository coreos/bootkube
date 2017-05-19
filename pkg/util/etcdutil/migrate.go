package etcdutil

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
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
	bootstrapEtcdServiceName = "bootstrap-etcd-service"
	etcdClusterName          = "kube-etcd"
)

var (
	pollInterval = 5 * time.Second
	pollTimeout  = 300 * time.Second
)

func Migrate(kubeConfig clientcmd.ClientConfig, svcPath, tprPath string) error {
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to create kube client config: %v", err)
	}
	kubecli, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kube client: %v", err)
	}
	restClient := kubecli.CoreV1().RESTClient()

	err = waitEtcdTPRReady(restClient, api.NamespaceSystem)
	if err != nil {
		return err
	}
	glog.Infof("created etcd cluster TPR")

	if err := createBootstrapEtcdService(kubecli, svcPath); err != nil {
		return fmt.Errorf("failed to create bootstrap-etcd-service: %v", err)
	}
	defer cleanupBootstrapEtcdService(kubecli)

	etcdServiceIP, err := getServiceIP(kubecli, api.NamespaceSystem, asset.EtcdServiceName)
	if err != nil {
		return err
	}
	glog.Infof("etcd-service IP is %s", etcdServiceIP)

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

func waitEtcdTPRReady(restClient restclient.Interface, ns string) error {
	err := wait.Poll(pollInterval, pollTimeout, func() (bool, error) {
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

func createBootstrapEtcdService(kubecli kubernetes.Interface, svcPath string) error {
	// Create the service.
	svc, err := ioutil.ReadFile(svcPath)
	if err != nil {
		return err
	}
	if err := kubecli.CoreV1().RESTClient().Post().RequestURI(fmt.Sprintf("/api/v1/namespaces/%s/services", api.NamespaceSystem)).SetHeader("Content-Type", "application/json").Body(svc).Do().Error(); err != nil {
		return err
	}

	// Wait for the service to be reachable (sometimes this takes a little while).
	if err := wait.Poll(pollInterval, pollTimeout, func() (bool, error) {
		svc, err := kubecli.CoreV1().Services(api.NamespaceSystem).Get(bootstrapEtcdServiceName, v1.GetOptions{})
		if err != nil {
			glog.Errorf("failed to get bootstrap etcd service: %v", err)
			return false, nil
		}
		resp, err := http.Get(fmt.Sprintf("http://%s:12379/version", svc.Spec.ClusterIP))
		if err != nil {
			glog.Infof("could not read bootstrap etcd version: %v", err)
			return false, nil
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if len(body) == 0 || err != nil {
			glog.Infof("could not read boot-etcd version: %v", err)
			return false, nil
		}
		return true, nil
	}); err != nil {
		return fmt.Errorf("timed out waiting for bootstrap etcd service: %s", err)
	}
	return nil
}

func createMigratedEtcdCluster(restclient restclient.Interface, tprPath string) error {
	tpr, err := ioutil.ReadFile(tprPath)
	if err != nil {
		return err
	}
	uri := fmt.Sprintf("/apis/%s/%s/namespaces/%s/%s", spec.TPRGroup, spec.TPRVersion, api.NamespaceSystem, spec.TPRKindPlural)
	return restclient.Post().RequestURI(uri).SetHeader("Content-Type", "application/json").Body(tpr).Do().Error()
}

func waitEtcdClusterRunning(restclient restclient.Interface) error {
	uri := fmt.Sprintf("/apis/%s/%s/namespaces/%s/%s/%s", spec.TPRGroup, spec.TPRVersion, api.NamespaceSystem, spec.TPRKindPlural, etcdClusterName)
	err := wait.Poll(pollInterval, pollTimeout, func() (bool, error) {
		b, err := restclient.Get().RequestURI(uri).DoRaw()
		if err != nil {
			glog.Errorf("failed to get etcd cluster TPR: %v", err)
			return false, nil
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
	err := wait.Poll(pollInterval, pollTimeout, func() (bool, error) {
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

func cleanupBootstrapEtcdService(kubecli kubernetes.Interface) {
	if err := wait.Poll(pollInterval, pollTimeout, func() (bool, error) {
		if err := kubecli.CoreV1().Services(api.NamespaceSystem).Delete(bootstrapEtcdServiceName, &v1.DeleteOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				glog.Info("bootstrap-etcd-service already removed")
				return true, nil
			}
			glog.Errorf("failed to remove bootstrap-etcd-service: %v", err)
			return false, nil
		}
		return true, nil
	}); err != nil {
		glog.Errorf("timed out removing bootstrap-etcd-service: %v", err)
	}
}
