package main

import (
	"fmt"
	"os"
	"time"

	"github.com/coreos/bootkube/pkg/node"
	"github.com/coreos/go-systemd/dbus"
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

const myPodNameEnvName = "MY_POD_NAME"

func main() {
	myPodName := os.Getenv(myPodNameEnvName)
	if myPodName == "" {
		glog.Fatalf("%s env var not set.", myPodNameEnvName)
	}

	client := newAPIClient()
	nodename, err := getNodeName(client, myPodName)
	if err != nil {
		glog.Fatal(err)
	}
	sysdConn, err := dbus.New()
	if err != nil {
		glog.Fatal(err)
	}
	defer sysdConn.Close()

	run(client, nodename, sysdConn)
}

func run(client clientset.Interface, nodename string, sysdConn *dbus.Conn) {
	glog.Infof("starting node agent, watching node: %s", nodename)

	a := node.Agent{
		NodeName: nodename,
		Client:   client,
		SysdConn: sysdConn,
	}

	nodeopts := api.ListOptions{
		FieldSelector: fields.ParseSelectorOrDie(fmt.Sprintf("metadata.name=%s", nodename)),
	}
	_, nodeController := framework.NewInformer(
		&cache.ListWatch{
			ListFunc: func(lo api.ListOptions) (runtime.Object, error) {
				return client.Core().Nodes().List(nodeopts)
			},
			WatchFunc: func(lo api.ListOptions) (watch.Interface, error) {
				return client.Core().Nodes().Watch(nodeopts)
			},
		},
		&v1.Node{},
		30*time.Second,
		framework.ResourceEventHandlerFuncs{
			UpdateFunc: a.NodeUpdateCallback,
		},
	)

	configMapStore, configMapController := framework.NewInformer(
		&cache.ListWatch{
			ListFunc: func(lo api.ListOptions) (runtime.Object, error) {
				return client.Core().ConfigMaps(api.NamespaceSystem).List(lo)
			},
			WatchFunc: func(lo api.ListOptions) (watch.Interface, error) {
				return client.Core().ConfigMaps(api.NamespaceSystem).Watch(lo)
			},
		},
		&v1.ConfigMap{},
		30*time.Second,
		framework.ResourceEventHandlerFuncs{},
	)

	a.ConfigMapStore = configMapStore

	go configMapController.Run(wait.NeverStop)
	nodeController.Run(wait.NeverStop)
}

func newAPIClient() clientset.Interface {
	config, err := restclient.InClusterConfig()
	if err != nil {
		glog.Fatal(err)
	}
	return clientset.NewForConfigOrDie(config)
}

func getNodeName(client clientset.Interface, myPodName string) (string, error) {
	p, err := client.Core().Pods(api.NamespaceSystem).Get(myPodName)
	if err != nil {
		return "", err
	}
	return p.Spec.NodeName, nil
}
