package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/api/v1"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/release_1_4"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	clientcmdapi "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"
)

const (
	kubeletAPIPodsURL = "http://127.0.0.1:10255/pods"
	ignorePath        = "/srv/kubernetes/manifests"
	activePath        = "/etc/kubernetes/manifests"
	kubeconfigPath    = "/etc/kubernetes/kubeconfig"
	secretsPath       = "/etc/kubernetes/checkpoint-secrets"

	kubeAPIServer = api.NamespaceSystem + "/" + "kube-apiserver"

	checkpointAnnotation   = "checkpoint.alpha.coreos.com/checkpoint"
	checkpointOfAnnotation = "checkpoint.alpha.coreos.com/checkpoint-of"
)

var podAPIServerMeta = unversioned.TypeMeta{
	APIVersion: "v1",
	Kind:       "Pod",
}

type checkpointPodPair struct {
	parent *v1.Pod
	child  *v1.Pod
}

var (
	secureAPIAddr = fmt.Sprintf("https://%s:%s", os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT_HTTPS"))
)

func main() {
	flag.Set("logtostderr", "true")
	defer glog.Flush()
	checkpoints, err := getCheckpointManifests()
	if err != nil {
		glog.Fatalf("failed to load existing checkpoint manifests: %v", err)
	}
	glog.Info("begin pods checkpointing...")
	run(checkpoints)
}

func run(checkpoints map[string]v1.Pod) {
	client := newAPIClient()
	for {
		var podList v1.PodList
		if err := json.Unmarshal(getPodsFromKubeletAPI(), &podList); err != nil {
			glog.Fatal(err)
		}

		pwc := getPodsWithCheckpointAnnotation(podList)
		// special case for handling API server readiness
		if _, ok := pwc[kubeAPIServer]; ok {
			// Make sure it's actually running. Sometimes we get that
			// pod manifest back, but the server is not actually running.
			if _, err := client.Discovery().ServerVersion(); err != nil {
				delete(pwc, kubeAPIServer)
			}
		}

		for upn, podpair := range pwc {
			switch {
			case podpair.parent != nil && podpair.child == nil:
				glog.Infof("only actual pod %v found, creating checkpoint pod manifest", upn)

				// The actual is running. Let's snapshot the pod,
				// clean it up a bit, and then save it to the ignore path for
				// later use.
				checkpointPod := createCheckpointPod(podpair.parent)
				convertSecretsToVolumeMounts(client, &checkpointPod)
				writeManifest(checkpointPod)
				checkpoints[upn] = checkpointPod
				glog.Infof("finished creating checkpoint pod %v manifest at %s\n", upn, checkpointManifest(upn))

			case podpair.parent != nil && podpair.child != nil:
				glog.Infof("both checkpoint and actual %v pods running, removing checkpoint pod", upn)
				// Both the temp and actual pods are running.
				// Remove the temp manifest from the config dir so that the
				// kubelet will stop it.
				if err := os.Remove(activeManifest(upn)); err != nil {
					glog.Error(err)
				}
			}
		}

		// start the checkpoint pod if it has not started yet
		for upn, m := range checkpoints {
			if _, ok := pwc[upn]; !ok {
				// attach `checkpoint of` annotation to pod to avoid endless looping
				// if no annotation attached to the static pod manifest.
				addCheckpointOfAnnotation(&m, upn)

				glog.Infof("no actual pod running, installing checkpoint pod static manifest")
				b, err := json.Marshal(m)
				if err != nil {
					glog.Error(err)
					continue
				}

				// Think: touch activeManifest first? do not write if already exists?

				if err := ioutil.WriteFile(activeManifest(upn), b, 0644); err != nil {
					glog.Error(err)
				}
			}
		}

		time.Sleep(1 * time.Second)
	}
}

func stripNonessentialInfo(p *v1.Pod) {
	p.Spec.ServiceAccountName = ""
	p.Spec.DeprecatedServiceAccount = ""
	p.Status.Reset()
}

func getPodsFromKubeletAPI() []byte {
	var pods []byte
	res, err := http.Get(kubeletAPIPodsURL)
	if err != nil {
		glog.Error(err)
		return pods
	}
	pods, err = ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		glog.Error(err)
	}
	return pods
}

// cleanVolumes will sanitize the list of volumes and volume mounts
// to remove the default service account token.
func cleanVolumes(p *v1.Pod) {
	volumes := make([]v1.Volume, 0, len(p.Spec.Volumes))
	for _, v := range p.Spec.Volumes {
		if !strings.HasPrefix(v.Name, "default-token") {
			volumes = append(volumes, v)
		}
	}
	p.Spec.Volumes = volumes
	for i := range p.Spec.Containers {
		c := &p.Spec.Containers[i]
		volumeMounts := make([]v1.VolumeMount, 0, len(c.VolumeMounts))
		for _, vm := range c.VolumeMounts {
			if !strings.HasPrefix(vm.Name, "default-token") {
				volumeMounts = append(volumeMounts, vm)
			}
		}
		c.VolumeMounts = volumeMounts
	}
}

// writeManifest will write the manifest to the ignore path.
// It first writes the file to a temp file, and then atomically moves it into
// the actual ignore path and correct file name.
func writeManifest(manifest v1.Pod) {
	m, err := json.Marshal(manifest)
	if err != nil {
		glog.Fatal(err)
	}
	writeAndAtomicCopy(m, checkpointManifest(uniquePodName(manifest)))
}

func createCheckpointPod(checkpointPod *v1.Pod) v1.Pod {
	upn := uniquePodName(*checkpointPod)

	cleanCheckpointPodMeta(checkpointPod)
	cleanVolumes(checkpointPod)
	stripNonessentialInfo(checkpointPod)
	addCheckpointOfAnnotation(checkpointPod, upn)
	return *checkpointPod
}

// WARNING: this copy is fragile. We need to figure out a better way
// to determine what to clean.
func cleanCheckpointPodMeta(cp *v1.Pod) {
	// the pod we manifest we got from kubelet does not have TypeMeta.
	// Add it now.
	cp.TypeMeta = podAPIServerMeta

	oldmeta := cp.ObjectMeta
	cp.ObjectMeta = v1.ObjectMeta{
		Name:      oldmeta.Name,
		Namespace: oldmeta.Namespace,
		Labels:    oldmeta.Labels,
	}
}

func addCheckpointOfAnnotation(cp *v1.Pod, upn string) {
	if cp.Annotations == nil {
		cp.Annotations = make(map[string]string)
	}
	cp.Annotations[checkpointOfAnnotation] = upn
}

func newAPIClient() clientset.Interface {
	kubeConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: secureAPIAddr}}).ClientConfig()
	if err != nil {
		glog.Fatal(err)
	}
	return clientset.NewForConfigOrDie(kubeConfig)
}

func convertSecretsToVolumeMounts(client clientset.Interface, pod *v1.Pod) {
	glog.Info("converting secrets to volume mounts")
	spec := pod.Spec
	for i := range spec.Volumes {
		v := &spec.Volumes[i]
		if v.Secret != nil {
			secretName := v.Secret.SecretName
			basePath := filepath.Join(secretsPath, pod.Name, v.Secret.SecretName)
			v.HostPath = &v1.HostPathVolumeSource{
				Path: basePath,
			}
			copySecretsToDisk(client, secretName, basePath)
			v.Secret = nil
		}
	}
}

func copySecretsToDisk(client clientset.Interface, secretName, basePath string) {
	glog.Info("copying secrets to disk")
	if err := os.MkdirAll(basePath, 0755); err != nil {
		glog.Fatal(err)
	}
	glog.Infof("created directory %s", basePath)
	s, err := client.Core().Secrets(api.NamespaceSystem).Get(secretName)
	if err != nil {
		glog.Fatal(err)
	}
	for name, value := range s.Data {
		path := filepath.Join(basePath, name)
		writeAndAtomicCopy(value, path)
	}
}

func writeAndAtomicCopy(data []byte, path string) {
	// First write a "temp" file.
	tmpfile := filepath.Join(filepath.Dir(path), "."+filepath.Base(path))
	if err := ioutil.WriteFile(tmpfile, data, 0644); err != nil {
		glog.Fatal(err)
	}
	// Finally, copy that file to the correct location.
	if err := os.Rename(tmpfile, path); err != nil {
		glog.Fatal(err)
	}
}

func activeManifest(name string) string {
	return filepath.Join(activePath, name+".json")
}

func checkpointManifest(name string) string {
	return filepath.Join(ignorePath, name+".json")
}

func getCheckpointManifests() (map[string]v1.Pod, error) {
	checkpoints := make(map[string]v1.Pod)

	fs, err := ioutil.ReadDir(ignorePath)
	if err != nil {
		if os.IsNotExist(err) {
			return checkpoints, nil
		}
		return nil, err
	}
	for _, f := range fs {
		glog.Infof("found checkpoint pod manifests at %v", f.Name())

		b, err := ioutil.ReadFile(path.Join(ignorePath, f.Name()))
		if err != nil {
			return nil, err
		}

		var manifest v1.Pod
		if err = json.Unmarshal(b, &manifest); err != nil {
			return nil, err
		}
		checkpoints[uniquePodName(manifest)] = manifest
	}
	return checkpoints, nil
}

func getPodsWithCheckpointAnnotation(pods v1.PodList) map[string]*checkpointPodPair {
	pa := make(map[string]*checkpointPodPair)

	for i := range pods.Items {
		p := pods.Items[i]

		// FIXME: trim off the pod name suffix to workaround with checkpointing the same
		// pod with different suffix multiple times on the same node.
		// A more reliable solution is to contact with the real API server to do GC.
		// See https://github.com/kubernetes-incubator/bootkube/pull/220#issuecomment-265616615
		// for more details.
		if strings.HasPrefix(p.GetName(), "kube-apiserver") {
			p.ObjectMeta.Name = "kube-apiserver"
		}
		if strings.HasPrefix(p.GetName(), "kube-etcd") {
			p.ObjectMeta.Name = "kube-etcd"
		}

		if n, ok := p.Annotations[checkpointAnnotation]; ok && n == "true" {
			upn := uniquePodName(p)
			if pa[upn] == nil {
				pa[upn] = &checkpointPodPair{}
			}
			pa[upn].parent = &p
		}

		if n, ok := p.Annotations[checkpointOfAnnotation]; ok {
			if pa[n] == nil {
				pa[n] = &checkpointPodPair{}
			}
			pa[n].child = &p
		}
	}

	return pa
}

func uniquePodName(p v1.Pod) string {
	return p.GetNamespace() + "-" + p.GetName()
}
