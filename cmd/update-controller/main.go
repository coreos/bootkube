package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/coreos/bootkube/pkg/cluster"
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

type versionPayload struct {
	newVersion *cluster.ContainerImage
	configMap  *v1.ConfigMap
}

func main() {
	config, err := restclient.InClusterConfig()
	if err != nil {
		glog.Fatal(err)
	}
	run(clientset.NewForConfigOrDie(config))
}

func run(client clientset.Interface) {
	glog.Info("update controller running")
	versionChan := make(chan *versionPayload)
	handleConfigChange := func(newConfigMap *v1.ConfigMap) {
		newVersion, err := parseNewVersion(newConfigMap)
		if err != nil {
			glog.Error(err)
			return
		}
		versionChan <- &versionPayload{newVersion: newVersion, configMap: newConfigMap}
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
	go func(client clientset.Interface) {
		for v := range versionChan {
			currentVersion, err := getCurrentVersion(client)
			if err != nil {
				glog.Error(err)
				return
			}
			if versionHasChanged(v.newVersion, currentVersion) {
				glog.Infof("new version: %s", v.newVersion)
				cu, err := cluster.NewClusterUpdater(client, v.newVersion, currentVersion, v.configMap)
				if err != nil {
					glog.Error(err)
					return
				}
				if err := cu.Update(); err != nil {
					glog.Error(err)
				}
			} else {
				glog.Info("no new version, nothing to do")
			}
		}
	}(client)
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

func parseNewVersion(config *v1.ConfigMap) (*cluster.ContainerImage, error) {
	version := config.Data[cluster.ClusterVersionKey]
	return parseContainerImageString(version)
}

func parseContainerImageString(str string) (*cluster.ContainerImage, error) {
	parts := strings.Split(str, ":")
	if len(parts) != 2 {
		return nil, fmt.Errorf("could not parse image: %s", str)
	}
	return &cluster.ContainerImage{
		Repo: parts[0],
		Tag:  parts[1],
	}, nil
}

// TODO: This could potentially be better acheived by storing this in the configmap object that
// we get the version from. If it does not exist, we can create it.
func getCurrentVersion(client clientset.Interface) (*cluster.ContainerImage, error) {
	apids, err := client.Extensions().DaemonSets(api.NamespaceSystem).Get("kube-apiserver")
	if err != nil {
		return nil, err
	}
	for _, c := range apids.Spec.Template.Spec.Containers {
		if c.Name == "kube-apiserver" {
			return parseContainerImageString(c.Image)
		}
	}
	return nil, errors.New("unable to get current cluster version")
}

func versionHasChanged(new, old *cluster.ContainerImage) bool {
	if new.Repo != old.Repo {
		return true
	}
	return new.Tag != old.Tag
}
