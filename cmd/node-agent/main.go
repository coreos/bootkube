package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

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

const (
	myPodNameEnvName        = "MY_POD_NAME"
	desiredConfigAnnotation = "node-agent.alpha.coreos.com/desired-config"
	currentConfigAnnotation = "node-agent.alpha.coreos.com/current-config"
	kubeletVersionKey       = "KUBELET_VERSION"
	kubeletConfigKey        = "KUBELET_CONFIG"
	kubeletFlagsKey         = "KUBELET_OPTS"
	kubeletEnvPath          = "/etc/kubernetes/kubelet.env"
	kubeletService          = "kubelet.service"
	configMapFlagsKey       = "kubelet-flags"
)

var (
	sysdConn       *dbus.Conn
	client         clientset.Interface
	configMapStore cache.Store

	myPodName = os.Getenv(myPodNameEnvName)
)

func main() {
	if myPodName == "" {
		glog.Fatalf("%s env var not set.", myPodNameEnvName)
	}
	run()
}

func run() {
	client = newAPIClient()
	nodename, err := getNodeName()
	if err != nil {
		glog.Fatal(err)
	}

	glog.Infof("starting node agent, watching node: %s", nodename)

	sysdConn, err = dbus.New()
	if err != nil {
		glog.Fatal(err)
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
			UpdateFunc: nodeUpdateCallback,
		},
	)
	var configMapController *framework.Controller
	configMapStore, configMapController = framework.NewInformer(
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

func nodeUpdateCallback(_, newObj interface{}) {
	glog.Infof("begin node update callback")

	newNode, ok := newObj.(*v1.Node)
	if !ok {
		glog.Errorf("received unexpected type: %T (expected *v1.Node)", newObj)
		return
	}
	onDiskConfig, err := parseKubeletEnvFile(kubeletEnvPath)
	if err != nil {
		glog.Error(err)
		return
	}

	if newNode.Annotations == nil {
		newNode.Annotations = make(map[string]string)
	}

	rawconf, ok := newNode.Annotations[desiredConfigAnnotation]
	if !ok {
		if _, ok := newNode.Annotations[currentConfigAnnotation]; !ok {
			setCurrentConfigAnnotation(newNode, onDiskConfig)
		}
		glog.Info("no %s annotation found for node %s, ignoring", desiredConfigAnnotation, newNode.Name)
		return
	}

	var desiredConfig map[string]string
	if err = json.Unmarshal([]byte(rawconf), &desiredConfig); err != nil {
		glog.Errorf("error unmarshaling config from %s: %v", desiredConfigAnnotation, err)
		return
	}
	if err = validateConfig(desiredConfig); err != nil {
		glog.Error(err)
		return
	}

	cm, err := getConfigMap(desiredConfig[kubeletConfigKey])
	if err != nil {
		glog.Error(err)
		return
	}
	if configHasChanged(onDiskConfig, desiredConfig, cm) {
		if err = updateKubeletEnvFile(desiredConfig, onDiskConfig, cm); err != nil {
			glog.Error(err)
			return
		}
		if err = restartKubeletService(); err != nil {
			glog.Error(err)
			return
		}
	}
	// Re-parse on disk config as it is source of truth.
	onDiskConfig, err = parseKubeletEnvFile(kubeletEnvPath)
	if err != nil {
		glog.Error(err)
		return
	}
	updatedConfig := make(map[string]string)
	updatedConfig[kubeletVersionKey] = onDiskConfig[kubeletVersionKey]
	updatedConfig[kubeletFlagsKey] = onDiskConfig[kubeletFlagsKey]
	updatedConfig[kubeletConfigKey] = desiredConfig[kubeletConfigKey]
	// Always update annotation at end of sync loop.
	if err := updateCurrentConfigNodeAnnotation(newNode, updatedConfig); err != nil {
		glog.Error(err)
	}
}

func setCurrentConfigAnnotation(node *v1.Node, onDiskConfig map[string]string) {
	currentConfig := make(map[string]string)
	currentConfig[kubeletVersionKey] = onDiskConfig[kubeletVersionKey]
	currentConfig[kubeletFlagsKey] = onDiskConfig[kubeletFlagsKey]
	currentConfig[kubeletConfigKey] = "on-disk configuration"
	if err := updateCurrentConfigNodeAnnotation(node, currentConfig); err != nil {
		glog.Error(err)
	}
}

func configHasChanged(onDiskConfig, desiredConfig map[string]string, cm *v1.ConfigMap) bool {
	if onDiskConfig[kubeletVersionKey] != desiredConfig[kubeletVersionKey] {
		return true
	}
	if onDiskConfig[kubeletFlagsKey] != cm.Data[configMapFlagsKey] {
		return true
	}
	return false
}

func validateConfig(desiredConfig map[string]string) error {
	if _, ok := desiredConfig[kubeletVersionKey]; !ok {
		return fmt.Errorf("configuration annotation does not contain required key: %s", kubeletVersionKey)
	}
	if _, ok := desiredConfig[kubeletConfigKey]; !ok {
		return fmt.Errorf("configuration annotation does not contain required key: %s", kubeletConfigKey)
	}
	return nil
}

func updateKubeletEnvFile(conf, onDiskConfig map[string]string, cm *v1.ConfigMap) error {
	var buf bytes.Buffer
	updatedConfig := make(map[string]string)
	for k, v := range onDiskConfig {
		updatedConfig[k] = v
	}
	updatedConfig[kubeletVersionKey] = conf[kubeletVersionKey]
	updatedConfig[kubeletFlagsKey] = cm.Data[configMapFlagsKey]
	for k, v := range updatedConfig {
		buf.WriteString(fmt.Sprintf("%s=%s\n", k, v))
	}
	return writeAndAtomicCopy(buf.Bytes(), kubeletEnvPath)
}

func parseKubeletEnvFile(path string) (map[string]string, error) {
	env := make(map[string]string)
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		kv := strings.SplitN(s.Text(), "=", 2)
		if len(kv) != 2 {
			glog.Infof("invalid config: %s", s.Text())
			continue
		}
		env[kv[0]] = kv[1]
	}
	return env, nil
}

func updateCurrentConfigNodeAnnotation(node *v1.Node, conf map[string]string) error {
	newConf, err := json.Marshal(conf)
	if err != nil {
		return fmt.Errorf("error attempting to marshal current config: %s", err)
	}
	node.Annotations[currentConfigAnnotation] = string(newConf)
	_, err = client.Core().Nodes().Update(node)
	return err
}

func restartKubeletService() error {
	// systemd daemon-reload equivalent
	err := sysdConn.Reload()
	if err != nil {
		return err
	}
	ch := make(chan string)
	_, err = sysdConn.RestartUnit(kubeletService, "replace", ch)
	if err != nil {
		return err
	}
	// Wait for unit to restart.
	result := <-ch
	if result != "done" {
		return fmt.Errorf("unexpected status received from systemd: %s", result)
	}
	return nil
}

func getConfigMap(configMapName string) (*v1.ConfigMap, error) {
	cmi, ok, err := configMapStore.GetByKey(fmt.Sprintf("%s/%s", api.NamespaceSystem, configMapName))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("configMap %s/%s does not exist", api.NamespaceSystem, configMapName)
	}
	cm, ok := cmi.(*v1.ConfigMap)
	if !ok {
		return nil, fmt.Errorf("received unexpected type: %T (expected *v1.ConfigMap)", cmi)
	}
	return cm, nil
}

func getNodeName() (string, error) {
	p, err := client.Core().Pods(api.NamespaceSystem).Get(myPodName)
	if err != nil {
		return "", err
	}
	return p.Spec.NodeName, nil
}

func writeAndAtomicCopy(data []byte, path string) error {
	// First write a "temp" file.
	tmpfile := filepath.Join(filepath.Dir(path), "."+filepath.Base(path))
	if err := ioutil.WriteFile(tmpfile, data, 0644); err != nil {
		return err
	}
	// Finally, copy that file to the correct location.
	return os.Rename(tmpfile, path)
}
