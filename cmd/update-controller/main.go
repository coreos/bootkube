package main

import (
	"os"
	"time"

	"github.com/coreos/bootkube/pkg/cluster"
	"github.com/coreos/bootkube/pkg/cluster/components"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/client/cache"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/release_1_3"
	"k8s.io/kubernetes/pkg/client/leaderelection"
	"k8s.io/kubernetes/pkg/client/restclient"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"
)

func main() {
	config, err := restclient.InClusterConfig()
	if err != nil {
		glog.Fatal(err)
	}
	c := clientset.NewForConfigOrDie(config)
	uc, err := client.New(config)
	if err != nil {
		glog.Fatal(err)
	}
	run(c, uc)
}

func run(c clientset.Interface, uc client.Interface) {
	glog.Info("update controller running")
	cu, err := cluster.NewClusterUpdater(c)
	if err != nil {
		glog.Error(err)
		return
	}
	handleConfigChange := func(newConfigMap *v1.ConfigMap) {
		newVersion, err := parseNewVersion(newConfigMap)
		if err != nil {
			glog.Error(err)
			return
		}
		if err := cu.UpdateToVersion(newVersion); err != nil {
			glog.Error(err)
		}
	}
	updateCallback := func(_, newObj interface{}) {
		glog.Info("begin update callback")
		newConfigMap, ok := newObj.(*v1.ConfigMap)
		if !ok {
			glog.Infof("Wrong type for update callback, expected *v1.ConfigMap, got: %T", newObj)
			return
		}
		handleConfigChange(newConfigMap)
	}
	addCallback := func(obj interface{}) {
		glog.Info("begin add callback")
		newConfigMap, ok := obj.(*v1.ConfigMap)
		if !ok {
			glog.Infof("Wrong type for add callback, expected *v1.ConfigMap, got: %T", obj)
			return
		}
		handleConfigChange(newConfigMap)
	}
	opts := api.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", cluster.ClusterConfigMapName),
	}
	_, configMapController := framework.NewInformer(
		&cache.ListWatch{
			ListFunc: func(lo api.ListOptions) (runtime.Object, error) {
				return c.Core().ConfigMaps(api.NamespaceSystem).List(opts)
			},
			WatchFunc: func(lo api.ListOptions) (watch.Interface, error) {
				return c.Core().ConfigMaps(api.NamespaceSystem).Watch(opts)
			},
		},
		&v1.ConfigMap{},
		30*time.Minute,
		framework.ResourceEventHandlerFuncs{
			AddFunc:    addCallback,
			UpdateFunc: updateCallback,
		},
	)

	stopChan := make(chan struct{})
	leaderelection.RunOrDie(leaderelection.LeaderElectionConfig{
		EndpointsMeta: api.ObjectMeta{
			Namespace: "kube-system",
			Name:      "kube-update-controller",
		},
		Client:   uc,
		Identity: os.Getenv("POD_NAME"),
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(stop <-chan struct{}) {
				go configMapController.Run(stopChan)
			},
			OnStoppedLeading: func() {
				stopChan <- struct{}{}
			},
		},
	})
}

func parseNewVersion(config *v1.ConfigMap) (*components.Version, error) {
	version := config.Data[cluster.ClusterVersionKey]
	return components.ParseVersionFromImage(version)
}
