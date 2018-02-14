package e2e

import (
	"testing"
	"time"

	"github.com/kubernetes-incubator/bootkube/e2e/internal/e2eutil/testworkload"
)

func TestSmoke(t *testing.T) {
	t.Skip()
	nginx, err := testworkload.NewNginx(client, namespace, testworkload.WithNginxPingJobLabels(map[string]string{"allow": "access"}))
	if err != nil {
		t.Fatalf("Test nginx creation failed: %v", err)
	}
	defer nginx.Delete()

	if err := retry(60, 5*time.Second, nginx.IsReachable); err != nil {
		t.Errorf("%s is not reachable: %v", nginx.Name, err)
	}
}
