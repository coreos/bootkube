package node

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/golang/glog"
	"github.com/kubernetes-incubator/bootkube/pkg/atomic"
	"github.com/kubernetes-incubator/bootkube/pkg/cluster/components/version"
	"k8s.io/kubernetes/pkg/api/v1"
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
}

type dbusConn interface {
	Reload() error
	RestartUnit(string, string, chan<- string) (int, error)
}

const (
	// DesiredVersionAnnotation is the annotation that describes the desired
	// version for the on-host kubelet.
	DesiredVersionAnnotation = "node-agent.alpha.coreos.com/desired-kubelet-version"
	// CurrentVersionAnnotation is the annotation that describes the current
	// version for the on-host kubelet.
	CurrentVersionAnnotation = "node-agent.alpha.coreos.com/current-kubelet-version"
	// KubeletVersionKey is the key used to lookup the kubelet version.
	KubeletVersionKey = "KUBELET_VERSION"
	// KubeletImageKey is the key used to lookup the kubelet image.
	KubeletImageKey = "KUBELET_ACI"

	defaultKubeletEnvPath = "/etc/kubernetes/kubelet.env"
	kubeletService        = "kubelet.service"
)

// NodeUpdateCallback is called via an informer watching Node objects.
// When called, it will check the Node annotations looking for a new desired
// version. When it finds a new desired version, it will update the on-disk config
// and restart the on-host kubelet.
func (a *Agent) NodeUpdateCallback(_, newObj interface{}) {
	glog.Info("begin node update callback")

	node, ok := newObj.(*v1.Node)
	if !ok {
		glog.Errorf("received unexpected type: %T (expected *v1.Node)", newObj)
		return
	}
	if node.Annotations == nil {
		node.Annotations = make(map[string]string)
	}
	onDiskConfig, err := parseKubeletEnvFile(defaultKubeletEnvPath)
	if err != nil {
		glog.Errorf("error parsing on-host kubelet env file: %v", err)
		return
	}
	desiredVersion, err := getDesiredVersion(node)
	if err != nil {
		glog.Error(err)
	}
	// Check desired config against the on disk config.
	if configHasChanged(onDiskConfig, desiredVersion) {
		err = a.handleConfigUpdate(onDiskConfig, desiredVersion, defaultKubeletEnvPath)
		if err != nil {
			glog.Error(err)
		}
	}
	if err := a.updateNode(node); err != nil {
		glog.Error(err)
	}
	glog.Info("node update callback finished")
}

func (a *Agent) handleConfigUpdate(onDiskConfig map[string]string, desiredVersion *version.Version, kubeletEnvPath string) error {
	// Update on disk config.
	err := updateKubeletEnvFile(kubeletEnvPath, desiredVersion, onDiskConfig)
	if err != nil {
		return err
	}
	// Restart kubelet via systemd to pick up new config.
	return a.restartKubeletService()
}

func (a *Agent) updateNode(node *v1.Node) error {
	// Always get information from the source of truth, which is the on disk config.
	onDiskConfig, err := parseKubeletEnvFile(defaultKubeletEnvPath)
	if err != nil {
		return fmt.Errorf("error parsing on-host kubelet env file: %v", err)
	}
	node.Annotations[CurrentVersionAnnotation] = onDiskConfig[KubeletVersionKey]
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

func getDesiredVersion(node *v1.Node) (*version.Version, error) {
	dv, ok := node.Annotations[DesiredVersionAnnotation]
	if !ok {
		err := fmt.Errorf("no %s annotation found for node %s, ignoring", DesiredVersionAnnotation, node.Name)
		return nil, err
	}
	return version.ParseFromImageString(dv)
}

func configHasChanged(onDiskConfig map[string]string, desiredVersion *version.Version) bool {
	if desiredVersion == nil {
		return false
	}
	if onDiskConfig[KubeletImageKey] != desiredVersion.Repo() {
		return true
	}
	return onDiskConfig[KubeletVersionKey] != desiredVersion.Tag()
}

func updateKubeletEnvFile(kubeletEnvPath string, version *version.Version, onDiskConfig map[string]string) error {
	var buf bytes.Buffer
	updatedConfig := make(map[string]string)
	for k, v := range onDiskConfig {
		updatedConfig[k] = v
	}
	updatedConfig[KubeletImageKey] = version.Repo()
	updatedConfig[KubeletVersionKey] = version.Tag()
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
