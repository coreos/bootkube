package e2e

import (
	"testing"

	"time"

	"github.com/coreos/ktestutil/testworkload"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	smokePingPollInterval = 1 * time.Second
	smokePingPollTimeout  = 1 * time.Minute
)

func TestSmoke(t *testing.T) {
	// 1. create nginx testworkload.
	var nginx *testworkload.Nginx
	if err := wait.Poll(networkPingPollInterval, networkPingPollTimeout, func() (bool, error) {
		var err error
		if nginx, err = testworkload.NewNginx(client, namespace, testworkload.WithNginxPingJobLabels(map[string]string{"allow": "access"})); err != nil {
			t.Logf("failed to create test nginx: %v", err)
			return false, nil
		}
		return true, nil
	}); err != nil {
		t.Fatalf("failed to create an testworkload: %v", err)
	}
	defer nginx.Delete()

	// 2. Create a wget pod that hits the nginx.
	if err := wait.Poll(smokePingPollInterval, smokePingPollTimeout, func() (bool, error) {
		if err := nginx.IsReachable(); err != nil {
			t.Logf("error not reachable %s: %v", nginx.Name, err)
			return false, nil
		}
		return true, nil
	}); err != nil {
		t.Fatalf("couldn't reach the test workload %s: %v", nginx.Name, err)
	}

}
