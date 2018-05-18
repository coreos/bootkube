package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/kubernetes-incubator/bootkube/e2e/internal/e2eutil/testworkload"
)

func TestSmoke(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	nginx, err := testworkload.NewNginx(ctx, client, namespace, testworkload.WithNginxPingJobLabels(map[string]string{"allow": "access"}))
	if err != nil {
		t.Fatalf("Test nginx creation failed: %v", err)
	}
	defer nginx.Delete(ctx)

	if err := retry(60, 5*time.Second, func() error {
		timeoutCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
		defer cancel()
		return nginx.IsReachable(timeoutCtx)
	}); err != nil {
		t.Errorf("%s is not reachable: %v", nginx.Name, err)
	}
}
