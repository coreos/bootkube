package recovery

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type apiServerBackend struct {
	client *kubernetes.Clientset
}

// NewAPIServerBackend constructs a new backend to talk to the API server using the given
// kubeConfig.
//
// TODO(diegs): support using a service account instead of a kubeconfig.
func NewAPIServerBackend(kubeConfigPath string) (Backend, error) {
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeConfigPath},
		&clientcmd.ConfigOverrides{})
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &apiServerBackend{
		client: client,
	}, nil
}

// read implements Backend.read().
func (b *apiServerBackend) read(context.Context) (*controlPlane, error) {
	cp := &controlPlane{}
	configMaps, err := b.client.CoreV1().ConfigMaps("kube-system").List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	cp.configMaps = *configMaps
	deployments, err := b.client.ExtensionsV1beta1().Deployments("kube-system").List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	cp.deployments = *deployments
	daemonSets, err := b.client.ExtensionsV1beta1().DaemonSets("kube-system").List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	cp.daemonSets = *daemonSets
	secrets, err := b.client.CoreV1().Secrets("kube-system").List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	cp.secrets = *secrets
	return cp, nil
}
