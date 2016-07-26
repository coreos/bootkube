package node

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/coreos/bootkube/pkg/atomic"
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
	SysdConn dbusConn
	// ConfigMapStore is updated via an informer.
	ConfigMapStore cache.Store
}

type dbusConn interface {
	Reload() error
	RestartUnit(string, string, chan<- string) (int, error)
}

const (
	// DesiredConfigAnnotation is the annotation that describes the desired
	// configuration state for the on-host kubelet.
	DesiredConfigAnnotation = "node-agent.alpha.coreos.com/desired-config"
	// CurrentConfigAnnotation is the annotation that describes the current
	// configuration state for the on-host kubelet.
	CurrentConfigAnnotation = "node-agent.alpha.coreos.com/current-config"
	// KubeletVersionKey is the key in the JSON stored in the config annotations that
	// describes the version of the kubelet to run on the host.
	KubeletVersionKey = "KUBELET_VERSION"
	// KubeletConfigKey is the key in the JSON stored in the config annotations that
	// describes the configuration flags for the kubelet to run on the host.
	KubeletConfigKey = "KUBELET_CONFIG"

	kubeletFlagsKey       = "KUBELET_OPTS"
	defaultKubeletEnvPath = "/etc/kubernetes/kubelet.env"
	kubeletService        = "kubelet.service"
	configMapFlagsKey     = "kubelet-flags"
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

	// Check annotations on Node.
	if node.Annotations == nil {
		node.Annotations = make(map[string]string)
	}

	var desiredConfig map[string]string

	onDiskConfig, err := parseKubeletEnvFile(defaultKubeletEnvPath)
	if err != nil {
		glog.Error(err)
		goto update
	}

	// Obtain and validate the desired configuration.
	desiredConfig, err = a.getDesiredConfig(node)
	if err != nil {
		glog.Error(err)
	}

update:
	updatedNode, err := a.handleConfigUpdate(*node, onDiskConfig, desiredConfig, defaultKubeletEnvPath)
	if err != nil {
		glog.Error(err)
	}

	// Always update annotation at end of sync loop.
	if err := a.updateNode(updatedNode); err != nil {
		glog.Error(err)
	}
}

func (a *Agent) handleConfigUpdate(node v1.Node, onDiskConfig, desiredConfig map[string]string, kubeletEnvPath string) (*v1.Node, error) {
	// `node` is already a copy, but just to make things clear, assign to new var
	newNode := node

	// Get the ConfigMap specified in the annotation from the local cache.
	cm, err := a.getConfigMap(desiredConfig[KubeletConfigKey])
	if err != nil {
		glog.Error(err)
	}

	err = validateConfig(desiredConfig)
	if err != nil {
		glog.Error(err)
	}

	// Check desired config / ConfigMap against the on disk config.
	if err == nil && configHasChanged(onDiskConfig, desiredConfig, cm) {
		// Update on disk config.
		onDiskConfig, err = updateKubeletEnvFile(kubeletEnvPath, desiredConfig, onDiskConfig, cm)
		if err != nil {
			// Log error but continue, we still want to update config.
			glog.Error(err)
		}
		// Restart kubelet via systemd to pick up new config.
		if err = a.restartKubeletService(); err != nil {
			// Log error but continue, we still want to update config.
			glog.Error(err)
		}
	}

	configName := desiredConfig[KubeletConfigKey]
	if configName == "" {
		configName = "on-disk configuration"
	}

	updatedConfig := make(map[string]string)
	updatedConfig[KubeletVersionKey] = onDiskConfig[KubeletVersionKey]
	updatedConfig[kubeletFlagsKey] = onDiskConfig[kubeletFlagsKey]
	updatedConfig[KubeletConfigKey] = configName
	newConf, err := json.Marshal(updatedConfig)
	if err != nil {
		return nil, fmt.Errorf("error attempting to marshal current config: %s", err)
	}
	newNode.Annotations[CurrentConfigAnnotation] = string(newConf)
	return &newNode, nil
}

func (a *Agent) getDesiredConfig(node *v1.Node) (map[string]string, error) {
	rawconf, ok := node.Annotations[DesiredConfigAnnotation]
	if !ok {
		err := fmt.Errorf("no %s annotation found for node %s, ignoring", DesiredConfigAnnotation, node.Name)
		return nil, err
	}

	var desiredConfig map[string]string
	if err := json.Unmarshal([]byte(rawconf), &desiredConfig); err != nil {
		return nil, fmt.Errorf("error unmarshaling config from %s: %v", DesiredConfigAnnotation, err)
	}
	return desiredConfig, nil
}

func (a *Agent) updateNode(node *v1.Node) error {
	_, err := a.Client.Core().Nodes().Update(node)
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
	if onDiskConfig[KubeletVersionKey] != desiredConfig[KubeletVersionKey] {
		return true
	}
	if cm == nil {
		return false
	}
	return onDiskConfig[kubeletFlagsKey] != cm.Data[configMapFlagsKey]
}

func validateConfig(desiredConfig map[string]string) error {
	if _, ok := desiredConfig[KubeletVersionKey]; !ok {
		return fmt.Errorf("configuration annotation does not contain required key: %s", KubeletVersionKey)
	}
	if _, ok := desiredConfig[KubeletConfigKey]; !ok {
		return fmt.Errorf("configuration annotation does not contain required key: %s", KubeletConfigKey)
	}
	return nil
}

func updateKubeletEnvFile(kubeletEnvPath string, conf, onDiskConfig map[string]string, cm *v1.ConfigMap) (map[string]string, error) {
	var buf bytes.Buffer
	updatedConfig := make(map[string]string)
	for k, v := range onDiskConfig {
		updatedConfig[k] = v
	}
	updatedConfig[KubeletVersionKey] = conf[KubeletVersionKey]
	if cm != nil {
		updatedConfig[kubeletFlagsKey] = cm.Data[configMapFlagsKey]
	}
	for k, v := range updatedConfig {
		buf.WriteString(fmt.Sprintf("%s=%s\n", k, v))
	}
	return updatedConfig, atomic.WriteAndCopy(buf.Bytes(), kubeletEnvPath)
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
