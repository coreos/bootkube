package checkpoint

import (
	"context"
	"time"

	"github.com/golang/glog"
	"google.golang.org/grpc"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kubernetes-sigs/bootkube/pkg/checkpoint/cri/v1alpha1"
	"github.com/kubernetes-sigs/bootkube/pkg/checkpoint/cri/v1alpha2"
	"github.com/kubernetes-sigs/bootkube/pkg/checkpoint/internal"
)

// Copied from "k8s.io/kubernetes/pkg/kubelet/types"
const (
	kubernetesPodNameLabel       = "io.kubernetes.pod.name"
	kubernetesPodNamespaceLabel  = "io.kubernetes.pod.namespace"
	kubernetesPodUIDLabel        = "io.kubernetes.pod.uid"
	kubernetesContainerNameLabel = "io.kubernetes.container.name"
	kubernetesContainerTypeLabel = "io.kubernetes.container.type"
)

type remoteRuntimeService struct {
	timeout        time.Duration
	v1alpha1Client v1alpha1.RuntimeServiceClient
	v1alpha2Client v1alpha2.RuntimeServiceClient
}

func newRemoteRuntimeService(endpoint string, connectionTimeout time.Duration) (*remoteRuntimeService, error) {
	glog.Infof("Connecting to runtime service %s", endpoint)
	addr, dialer, err := internal.GetAddressAndDialer(endpoint)
	if err != nil {
		return nil, err
	}

	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithTimeout(connectionTimeout), grpc.WithContextDialer(dialer))
	if err != nil {
		glog.Errorf("Connect remote runtime %s failed: %v", addr, err)
		return nil, err
	}

	return &remoteRuntimeService{
		timeout:        connectionTimeout,
		v1alpha1Client: v1alpha1.NewRuntimeServiceClient(conn),
		v1alpha2Client: v1alpha2.NewRuntimeServiceClient(conn),
	}, nil
}

// localRunningPods uses the CRI shim to retrieve the local container runtime pod state
func (r *remoteRuntimeService) localRunningPods() map[string]*v1.Pod {
	pods := make(map[string]*v1.Pod)

	// Retrieving sandboxes is likely redundant but is done to maintain sameness with what the kubelet does
	sandboxes, err := r.getRunningKubeletSandboxes()
	if err != nil {
		glog.Errorf("failed to list running sandboxes: %v", err)
		return nil
	}

	// Add pods from all sandboxes
	for _, s := range sandboxes {
		podName := s.Namespace + "/" + s.Name
		if _, ok := pods[podName]; !ok {
			p := &v1.Pod{}
			p.UID = types.UID(s.Uid)
			p.Name = s.Name
			p.Namespace = s.Namespace

			pods[podName] = p
		}
	}

	containers, err := r.getRunningKubeletContainers()
	if err != nil {
		glog.Errorf("failed to list running containers: %v", err)
		return nil
	}

	// Add all pods that containers are apart of
	for _, c := range containers {

		podName := c.Labels[kubernetesPodNamespaceLabel] + "/" + c.Labels[kubernetesPodNameLabel]
		if _, ok := pods[podName]; !ok {
			p := &v1.Pod{}
			p.UID = types.UID(c.Labels[kubernetesPodUIDLabel])
			p.Name = c.Labels[kubernetesPodNameLabel]
			p.Namespace = c.Labels[kubernetesPodNamespaceLabel]

			pods[podName] = p
		}
	}

	return pods
}

type criContainer struct {
	Labels map[string]string
}

type criSandbox struct {
	Uid       string
	Name      string
	Namespace string
	Labels    map[string]string
}

func (r *remoteRuntimeService) getRunningKubeletContainers() ([]criContainer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	var containers []criContainer
	if _, err := r.v1alpha1Client.Version(ctx, &v1alpha1.VersionRequest{}); err == nil {
		resp, err := r.v1alpha1Client.ListContainers(ctx, &v1alpha1.ListContainersRequest{
			Filter: &v1alpha1.ContainerFilter{
				State: &v1alpha1.ContainerStateValue{
					// Filter out non-running containers
					State: v1alpha1.ContainerState_CONTAINER_RUNNING,
				},
			},
		})
		if err != nil {
			glog.Errorf("ListContainers with filter from runtime service failed: %v", err)
			return nil, err
		}
		for _, c := range resp.Containers {
			if c.Metadata == nil {
				glog.V(4).Infof("Container does not have metadata: %+v", c)
				continue
			}
			containers = append(containers, criContainer{Labels: c.Labels})
		}
		return containers, nil
	}

	// Try v1alpha2
	resp, err := r.v1alpha2Client.ListContainers(ctx, &v1alpha2.ListContainersRequest{
		Filter: &v1alpha2.ContainerFilter{
			State: &v1alpha2.ContainerStateValue{
				// Filter out non-running containers
				State: v1alpha2.ContainerState_CONTAINER_RUNNING,
			},
		},
	})
	if err != nil {
		glog.Errorf("ListContainers with filter from runtime service failed: %v", err)
		return nil, err
	}
	for _, c := range resp.Containers {
		if c.Metadata == nil {
			glog.V(4).Infof("Container does not have metadata: %+v", c)
			continue
		}
		containers = append(containers, criContainer{Labels: c.Labels})
	}
	return containers, nil
}

func (r *remoteRuntimeService) getRunningKubeletSandboxes() ([]criSandbox, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	var sandboxes []criSandbox
	if _, err := r.v1alpha1Client.Version(ctx, &v1alpha1.VersionRequest{}); err == nil {
		resp, err := r.v1alpha1Client.ListPodSandbox(ctx, &v1alpha1.ListPodSandboxRequest{
			Filter: &v1alpha1.PodSandboxFilter{
				// Filter out non-running sandboxes
				State: &v1alpha1.PodSandboxStateValue{
					State: v1alpha1.PodSandboxState_SANDBOX_READY,
				},
			},
		})
		if err != nil {
			glog.Errorf("ListPodSandbox with filter from runtime service failed: %v", err)
			return nil, err
		}
		for _, c := range resp.Items {
			if c.Metadata == nil {
				glog.V(4).Infof("Sandbox does not have metadata: %+v", c)
				continue
			}
			sandboxes = append(sandboxes, criSandbox{
				Uid:       c.Metadata.Uid,
				Name:      c.Metadata.Name,
				Namespace: c.Metadata.Namespace,
				Labels:    c.Labels,
			})
		}
		return sandboxes, nil
	}
	resp, err := r.v1alpha2Client.ListPodSandbox(ctx, &v1alpha2.ListPodSandboxRequest{
		Filter: &v1alpha2.PodSandboxFilter{
			// Filter out non-running sandboxes
			State: &v1alpha2.PodSandboxStateValue{
				State: v1alpha2.PodSandboxState_SANDBOX_READY,
			},
		},
	})
	if err != nil {
		glog.Errorf("ListPodSandbox with filter from runtime service failed: %v", err)
		return nil, err
	}
	for _, c := range resp.Items {
		if c.Metadata == nil {
			glog.V(4).Infof("Sandbox does not have metadata: %+v", c)
			continue
		}
		sandboxes = append(sandboxes, criSandbox{
			Uid:       c.Metadata.Uid,
			Name:      c.Metadata.Name,
			Namespace: c.Metadata.Namespace,
			Labels:    c.Labels,
		})
	}
	return sandboxes, nil
}
