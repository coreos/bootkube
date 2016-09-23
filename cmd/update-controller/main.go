package main

import (
	"os"
	"time"

	"github.com/kubernetes-incubator/bootkube/pkg/cluster"
	"github.com/kubernetes-incubator/bootkube/pkg/cluster/components"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/leaderelection"
	"k8s.io/kubernetes/pkg/client/record"
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
	uc, err := client.New(config)
	if err != nil {
		glog.Fatal(err)
	}
	internalclient := internalclientset.NewForConfigOrDie(config)
	run(uc, internalclient)
}

func run(uc client.Interface, internalclient internalclientset.Interface) {
	glog.Info("update controller running")
	cu, err := cluster.NewClusterUpdater(uc, internalclient)
	if err != nil {
		glog.Error(err)
		return
	}
	handleConfigChange := func(newConfigMap *api.ConfigMap) {
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
		newConfigMap, ok := newObj.(*api.ConfigMap)
		if !ok {
			glog.Infof("Wrong type for update callback, expected *v1.ConfigMap, got: %T", newObj)
			return
		}
		handleConfigChange(newConfigMap)
	}
	addCallback := func(obj interface{}) {
		glog.Info("begin add callback")
		newConfigMap, ok := obj.(*api.ConfigMap)
		if !ok {
			glog.Infof("Wrong type for add callback, expected *v1.ConfigMap, got: %T", obj)
			return
		}
		handleConfigChange(newConfigMap)
	}
	opts := api.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", cluster.ClusterConfigMapName),
	}
	// TODO Once we switch to Third Party Resources, we should skip the informer,
	// and from there move to a simple polling solution that checks all
	// version 3rd party resource objects.
	_, configMapController := framework.NewInformer(
		&cache.ListWatch{
			ListFunc: func(lo api.ListOptions) (runtime.Object, error) {
				return uc.ConfigMaps(api.NamespaceSystem).List(opts)
			},
			WatchFunc: func(lo api.ListOptions) (watch.Interface, error) {
				return uc.ConfigMaps(api.NamespaceSystem).Watch(opts)
			},
		},
		&api.ConfigMap{},
		30*time.Second,
		framework.ResourceEventHandlerFuncs{
			AddFunc:    addCallback,
			UpdateFunc: updateCallback,
		},
	)

	b := record.NewBroadcaster()
	r := b.NewRecorder(api.EventSource{
		Component: "cluster-updater",
	})
	leaderelection.RunOrDie(leaderelection.LeaderElectionConfig{
		EndpointsMeta: api.ObjectMeta{
			Namespace: "kube-system",
			Name:      "kube-update-controller",
		},
		Client:        uc,
		Identity:      os.Getenv("POD_NAME"),
		LeaseDuration: 15 * time.Second,
		RenewDeadline: 10 * time.Second,
		RetryPeriod:   2 * time.Second,
		EventRecorder: r,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(stop <-chan struct{}) {
				glog.Info("started leading: running update controller")
				go configMapController.Run(stop)
			},
			OnStoppedLeading: func() {
				glog.Fatal("stopped leading: pausing update controller")
			},
		},
	})
}

func parseNewVersion(config *api.ConfigMap) (*components.Version, error) {
	version := config.Data[cluster.ClusterVersionKey]
	return components.ParseVersionFromImage(version)
}
