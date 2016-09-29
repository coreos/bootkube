package main

import (
	"os"
	"time"

	"github.com/kubernetes-incubator/bootkube/pkg/cluster"

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
	"k8s.io/kubernetes/pkg/util/wait"
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

func run(client client.Interface, internalclient internalclientset.Interface) {
	glog.Info("update controller running")
	opts := api.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", cluster.ClusterConfigMapName),
	}
	// TODO switch to 3rd Party Resource.
	configMapStore, configMapController := framework.NewInformer(
		&cache.ListWatch{
			ListFunc: func(lo api.ListOptions) (runtime.Object, error) {
				return client.ConfigMaps(api.NamespaceSystem).List(opts)
			},
			WatchFunc: func(lo api.ListOptions) (watch.Interface, error) {
				return client.ConfigMaps(api.NamespaceSystem).Watch(opts)
			},
		},
		&api.ConfigMap{},
		30*time.Second,
		framework.ResourceEventHandlerFuncs{},
	)
	go configMapController.Run(wait.NeverStop)

	uc, err := cluster.NewUpdateController(client, internalclient, configMapStore)
	if err != nil {
		glog.Error(err)
		return
	}

	b := record.NewBroadcaster()
	r := b.NewRecorder(api.EventSource{
		Component: "cluster-updater",
	})
	leaderelection.RunOrDie(leaderelection.LeaderElectionConfig{
		EndpointsMeta: api.ObjectMeta{
			Namespace: "kube-system",
			Name:      "kube-update-controller",
		},
		Client:        client,
		Identity:      os.Getenv("POD_NAME"),
		LeaseDuration: 15 * time.Second,
		RenewDeadline: 10 * time.Second,
		RetryPeriod:   2 * time.Second,
		EventRecorder: r,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(stop <-chan struct{}) {
				glog.Info("started leading: running update controller")
				go uc.Run(stop)
			},
			OnStoppedLeading: func() {
				glog.Fatal("stopped leading: pausing update controller")
			},
		},
	})
}
