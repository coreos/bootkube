package checkpoint

import (
	"context"
	"time"

	"github.com/golang/glog"
	"google.golang.org/grpc"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
	kubelettypes "k8s.io/kubernetes/pkg/kubelet/types"
	"k8s.io/kubernetes/pkg/kubelet/util"
)

type remoteRuntimeService struct {
	timeout       time.Duration
	runtimeClient runtimeapi.RuntimeServiceClient
}

func newRemoteRuntimeService(endpoint string, connectionTimeout time.Duration) (*remoteRuntimeService, error) {
	glog.Infof("Connecting to runtime service %s", endpoint)
	addr, dialer, err := util.GetAddressAndDialer(endpoint)
	if err != nil {
		return nil, err
	}

	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithTimeout(connectionTimeout), grpc.WithDialer(dialer))
	if err != nil {
		glog.Errorf("Connect remote runtime %s failed: %v", addr, err)
		return nil, err
	}

	return &remoteRuntimeService{
		timeout:       connectionTimeout,
		runtimeClient: runtimeapi.NewRuntimeServiceClient(conn),
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
		if s.Metadata == nil {
			glog.V(4).Infof("Sandbox does not have metadata: %+v", s)
			continue
		}

		podName := s.Metadata.Namespace + "/" + s.Metadata.Name
		if _, ok := pods[podName]; !ok {
			p := &v1.Pod{}
			p.UID = types.UID(s.Metadata.Uid)
			p.Name = s.Metadata.Name
			p.Namespace = s.Metadata.Namespace

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
		if c.Metadata == nil {
			glog.V(4).Infof("Container does not have metadata: %+v", c)
			continue
		}

		podName := c.Labels[kubelettypes.KubernetesPodNamespaceLabel] + "/" + c.Labels[kubelettypes.KubernetesPodNameLabel]
		if _, ok := pods[podName]; !ok {
			p := &v1.Pod{}
			p.UID = types.UID(c.Labels[kubelettypes.KubernetesPodUIDLabel])
			p.Name = c.Labels[kubelettypes.KubernetesPodNameLabel]
			p.Namespace = c.Labels[kubelettypes.KubernetesPodNamespaceLabel]

			pods[podName] = p
		}
	}

	return pods
}

func (r *remoteRuntimeService) getRunningKubeletContainers() ([]*runtimeapi.Container, error) {
	filter := &runtimeapi.ContainerFilter{}

	// Filter out non-running containers
	filter.State = &runtimeapi.ContainerStateValue{
		State: runtimeapi.ContainerState_CONTAINER_RUNNING,
	}

	return r.listContainers(filter)
}

func (r *remoteRuntimeService) getRunningKubeletSandboxes() ([]*runtimeapi.PodSandbox, error) {
	filter := &runtimeapi.PodSandboxFilter{}

	// Filter out non-running sandboxes
	filter.State = &runtimeapi.PodSandboxStateValue{
		State: runtimeapi.PodSandboxState_SANDBOX_READY,
	}
	return r.listPodSandbox(filter)
}

func (r *remoteRuntimeService) listPodSandbox(filter *runtimeapi.PodSandboxFilter) ([]*runtimeapi.PodSandbox, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	resp, err := r.runtimeClient.ListPodSandbox(ctx, &runtimeapi.ListPodSandboxRequest{
		Filter: filter,
	})
	if err != nil {
		glog.Errorf("ListPodSandbox with filter %q from runtime sevice failed: %v", filter, err)
		return nil, err
	}

	return resp.Items, nil
}

func (r *remoteRuntimeService) listContainers(filter *runtimeapi.ContainerFilter) ([]*runtimeapi.Container, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	resp, err := r.runtimeClient.ListContainers(ctx, &runtimeapi.ListContainersRequest{
		Filter: filter,
	})
	if err != nil {
		glog.Errorf("ListContainers with filter %q from runtime service failed: %v", filter, err)
		return nil, err
	}

	return resp.Containers, nil
}
