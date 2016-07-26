package cluster

import (
	"errors"
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/release_1_3"
	"k8s.io/kubernetes/pkg/client/record"
)

func successfulUpdate(_ clientset.Interface, _ string, _ *ContainerImage) error {
	return nil
}

func failedUpdate(_ clientset.Interface, _ string, _ *ContainerImage) error {
	return errors.New("fail")
}

func TestUpdateCluster(t *testing.T) {
	testcases := []struct {
		list             []ComponentUpdater
		expectedRollback bool
	}{
		{
			list:             []ComponentUpdater{{UpdateFn: successfulUpdate}},
			expectedRollback: false,
		},
		{
			list:             []ComponentUpdater{{UpdateFn: successfulUpdate}, {UpdateFn: failedUpdate}},
			expectedRollback: true,
		},
	}
	var didrollback bool
	fakerollbackfn := func(clientset.Interface, *ContainerImage, *v1.ConfigMap) error {
		didrollback = true
		return nil
	}
	for _, tc := range testcases {
		didrollback = false
		b := record.NewBroadcaster()
		r := b.NewRecorder(api.EventSource{Component: "cluster-updater-test"})
		cu := &ClusterUpdater{
			Components: tc.list,
			RollbackFn: fakerollbackfn,
			Events:     r,
		}
		cu.Update()
		if didrollback != tc.expectedRollback {
			t.Fatal("expected cluster update to roll back, it did not")
		}
	}
}
