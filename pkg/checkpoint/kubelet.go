package checkpoint

import (
	"fmt"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

// A minimal kubelet client. It assumes the kubelet can be reached the kubelet's insecure API at
// 127.0.0.1:10255 and the secure API at 127.0.0.1:10250.
type kubeletClient struct {
	insecureClient *rest.RESTClient
	secureClient   *rest.RESTClient
}

func newKubeletClient(config *rest.Config) (*kubeletClient, error) {
	// Use the core API group serializer. Same logic as client-go.
	// https://github.com/kubernetes/client-go/blob/v3.0.0/kubernetes/typed/core/v1/core_client.go#L147
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}

	// Kubelet is using a self-signed cert.
	config.TLSClientConfig.Insecure = true
	config.TLSClientConfig.CAFile = ""
	config.TLSClientConfig.CAData = nil

	// Shallow copy.
	insecureConfig := *config
	secureConfig := *config

	insecureConfig.Host = "http://127.0.0.1:10255"
	secureConfig.Host = "https://127.0.0.1:10250"

	client := new(kubeletClient)
	var err error
	if client.insecureClient, err = rest.UnversionedRESTClientFor(&insecureConfig); err != nil {
		return nil, fmt.Errorf("failed creating kubelet client for debug endpoints: %v", err)
	}
	if client.secureClient, err = rest.UnversionedRESTClientFor(&secureConfig); err != nil {
		return nil, fmt.Errorf("failed creating kubelet client: %v", err)
	}

	return client, nil
}

// localParentPods will retrieve all pods from kubelet api that are parents & should be checkpointed
func (k *kubeletClient) localParentPods() map[string]*v1.Pod {
	podList := new(v1.PodList)
	if err := k.insecureClient.Get().AbsPath("/pods/").Do().Into(podList); err != nil {
		// Assume there are no local parent pods.
		glog.Errorf("failed to list local parent pods, assuming none are running: %v", err)
	}
	return podListToParentPods(podList)
}
