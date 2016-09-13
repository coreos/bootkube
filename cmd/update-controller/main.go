package main

import (
	"fmt"
	"time"

	"github.com/coreos/bootkube/pkg/cluster"
	"github.com/coreos/bootkube/pkg/cluster/components"
	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/client/cache"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/release_1_3"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/watch"
)

func main() {
	config, err := restclient.InClusterConfig()
	if err != nil {
		glog.Fatal(err)
	}
	run(clientset.NewForConfigOrDie(config))
}

func run(client clientset.Interface) {
	glog.Info("update controller running")
	versionChan := make(chan *components.Version)
	handleConfigChange := func(newConfigMap *v1.ConfigMap) {
		newVersion, err := parseNewVersion(newConfigMap)
		if err != nil {
			glog.Error(err)
			return
		}
		versionChan <- newVersion
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
			glog.Infof("Wrong type for update callback, expected *v1.ConfigMap, got: %T", obj)
			return
		}
		handleConfigChange(newConfigMap)
	}
	cu, err := cluster.NewClusterUpdater(client)
	if err != nil {
		glog.Error(err)
		return
	}
	go func(client clientset.Interface, updater *cluster.ClusterUpdater) {
		for v := range versionChan {
			if err := updater.UpdateToVersion(v); err != nil {
				glog.Error(err)
			}
		}
	}(client, cu)
	opts := api.ListOptions{
		FieldSelector: fields.ParseSelectorOrDie(fmt.Sprintf("metadata.name=%s", cluster.ClusterConfigMapName)),
	}
	_, configMapController := framework.NewInformer(
		&cache.ListWatch{
			ListFunc: func(lo api.ListOptions) (runtime.Object, error) {
				return client.Core().ConfigMaps(api.NamespaceSystem).List(opts)
			},
			WatchFunc: func(lo api.ListOptions) (watch.Interface, error) {
				return client.Core().ConfigMaps(api.NamespaceSystem).Watch(opts)
			},
		},
		&v1.ConfigMap{},
		30*time.Minute,
		framework.ResourceEventHandlerFuncs{
			AddFunc:    addCallback,
			UpdateFunc: updateCallback,
		},
	)
	configMapController.Run(wait.NeverStop)
	close(versionChan)
}

func parseNewVersion(config *v1.ConfigMap) (*components.Version, error) {
	version := config.Data[cluster.ClusterVersionKey]
	return components.ParseVersionFromImage(version)
}
