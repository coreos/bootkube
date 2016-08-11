package node

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/coreos/bootkube/pkg/atomic"
	"github.com/coreos/go-systemd/dbus"
	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/client/cache"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/release_1_3"
)

// Agent reacts to annotation updates on the node it is running on.
// When the annotation regarding the desired config is updated,
// the Agent will update the configuration on disk and restart the
// on-host kubelet.
type Agent struct {
	// NodeName is the name of the node the agent is running on.
	NodeName string
	// Client is an API server client.
	Client clientset.Interface
	// SysdConn is a sytemd dbus connection, used for restarting the on-host
	// kubelet via systemd.
	SysdConn *dbus.Conn
	// ConfigMapStore is updated via an informer.
	ConfigMapStore cache.Store
}

const (
	desiredConfigAnnotation = "node-agent.alpha.coreos.com/desired-config"
	currentConfigAnnotation = "node-agent.alpha.coreos.com/current-config"
	kubeletVersionKey       = "KUBELET_VERSION"
	kubeletConfigKey        = "KUBELET_CONFIG"
	kubeletFlagsKey         = "KUBELET_OPTS"
	kubeletEnvPath          = "/etc/kubernetes/kubelet.env"
	kubeletService          = "kubelet.service"
	configMapFlagsKey       = "kubelet-flags"
)

// NodeUpdateCallback is called via an informer watching Node objects.
// When called, it will check the Node annotations looking for a new desired
// config. When it finds a new desired config, it will update the on-disk config
// and restart the on-host kubelet.
func (a *Agent) NodeUpdateCallback(_, newObj interface{}) {
	glog.Infof("begin node update callback")

	node, ok := newObj.(*v1.Node)
	if !ok {
		glog.Errorf("received unexpected type: %T (expected *v1.Node)", newObj)
		return
	}

	onDiskConfig, err := parseKubeletEnvFile(kubeletEnvPath)
	if err != nil {
		glog.Error(err)
		return
	}

	// Check annotations on Node.
	if node.Annotations == nil {
		node.Annotations = make(map[string]string)
	}
	// If we don't have a current config, set it to the value of the
	// on disk config. This gives insight to cluster admins, and allows
	// them to use the data in this annotation to update the config
	// via configmaps.
	if _, ok := node.Annotations[currentConfigAnnotation]; !ok {
		a.setCurrentConfigAnnotation(node, onDiskConfig)
	}

	// Obtain and validate the desired configuration.
	desiredConfig, err := a.getDesiredConfig(node)
	if err != nil {
		glog.Error(err)
		return
	}
	if err = validateConfig(desiredConfig); err != nil {
		glog.Error(err)
		return
	}

	// Get the ConfigMap specified in the annotation from the API server.
	cm, err := a.getConfigMap(desiredConfig[kubeletConfigKey])
	if err != nil {
		glog.Error(err)
		return
	}

	// Check desired config / ConfigMap against the on disk config.
	if configHasChanged(onDiskConfig, desiredConfig, cm) {
		// Update on disk config.
		if err = updateKubeletEnvFile(desiredConfig, onDiskConfig, cm); err != nil {
			glog.Error(err)
			return
		}
		// Restart kubelet via systemd to pick up new config.
		if err = a.restartKubeletService(); err != nil {
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

	// Always update annotation at end of sync loop.
	updatedConfig := make(map[string]string)
	updatedConfig[kubeletVersionKey] = onDiskConfig[kubeletVersionKey]
	updatedConfig[kubeletFlagsKey] = onDiskConfig[kubeletFlagsKey]
	updatedConfig[kubeletConfigKey] = desiredConfig[kubeletConfigKey]
	if err := a.updateCurrentConfigNodeAnnotation(node, updatedConfig); err != nil {
		glog.Error(err)
	}
}

func (a *Agent) getDesiredConfig(node *v1.Node) (map[string]string, error) {
	rawconf, ok := node.Annotations[desiredConfigAnnotation]
	if !ok {
		err := fmt.Errorf("no %s annotation found for node %s, ignoring", desiredConfigAnnotation, node.Name)
		return nil, err
	}

	var desiredConfig map[string]string
	if err := json.Unmarshal([]byte(rawconf), &desiredConfig); err != nil {
		return nil, fmt.Errorf("error unmarshaling config from %s: %v", desiredConfigAnnotation, err)
	}
	return desiredConfig, nil
}

func (a *Agent) setCurrentConfigAnnotation(node *v1.Node, onDiskConfig map[string]string) {
	currentConfig := make(map[string]string)
	currentConfig[kubeletVersionKey] = onDiskConfig[kubeletVersionKey]
	currentConfig[kubeletFlagsKey] = onDiskConfig[kubeletFlagsKey]
	currentConfig[kubeletConfigKey] = "on-disk configuration"
	if err := a.updateCurrentConfigNodeAnnotation(node, currentConfig); err != nil {
		glog.Error(err)
	}
}

func (a *Agent) updateCurrentConfigNodeAnnotation(node *v1.Node, conf map[string]string) error {
	newConf, err := json.Marshal(conf)
	if err != nil {
		return fmt.Errorf("error attempting to marshal current config: %s", err)
	}
	node.Annotations[currentConfigAnnotation] = string(newConf)
	_, err = a.Client.Core().Nodes().Update(node)
	return err
}

func (a *Agent) restartKubeletService() error {
	// systemd daemon-reload equivalent
	err := a.SysdConn.Reload()
	if err != nil {
		return err
	}
	ch := make(chan string)
	_, err = a.SysdConn.RestartUnit(kubeletService, "replace", ch)
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

func (a *Agent) getConfigMap(configMapName string) (*v1.ConfigMap, error) {
	cmi, ok, err := a.ConfigMapStore.GetByKey(fmt.Sprintf("%s/%s", api.NamespaceSystem, configMapName))
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
	return atomic.WriteAndCopy(buf.Bytes(), kubeletEnvPath)
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
