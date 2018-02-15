package e2e

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kubernetes-incubator/bootkube/e2e/internal/e2eutil/testworkload"

	"k8s.io/api/extensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
)

const (
	networkPingPollInterval = 1 * time.Second
	networkPingPollTimeout  = 5 * time.Minute
)

// 1. create nginx test workload.
// 2. check if network is setup right.
// 3. set DefaultDeny policy
// 4. create a wget job that fails to hit nginx service
// 5. create NetworkPolicy that allows `allow=access`
// 6. create a wget job with label `allow=access` that hits the nginx service
func TestNetwork(t *testing.T) {
	// check if calico-node daemonset exists
	// if absent skip this test
	if _, err := client.ExtensionsV1beta1().DaemonSets("kube-system").Get("calico-node", metav1.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			t.Skip("calico-node daemonset is not installed")
		}
		t.Fatalf("error getting calico-node daemonset: %v", err)
	}

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

	if err := wait.Poll(networkPingPollInterval, networkPingPollTimeout, func() (bool, error) {
		if err := nginx.IsReachable(); err != nil {
			t.Logf("error not reachable %s: %v", nginx.Name, err)
			return false, nil
		}
		return true, nil
	}); err != nil {
		t.Fatalf("network not set up correctly: %v", err)
	}

	t.Run("DefaultDeny", func(t *testing.T) { HelperDefaultDeny(t, nginx) })
	t.Run("NetworkPolicy", func(t *testing.T) { HelperPolicy(t, nginx) })
}

func HelperDefaultDeny(t *testing.T, nginx *testworkload.Nginx) {
	npi, _, err := legacyscheme.Codecs.UniversalDecoder().Decode(defaultDenyNetworkPolicy, nil, &v1beta1.NetworkPolicy{})
	if err != nil {
		t.Fatalf("unable to decode network policy manifest: %v", err)
	}
	np, ok := npi.(*v1beta1.NetworkPolicy)
	if !ok {
		t.Fatalf("expected manifest to decode into *api.networkpolicy, got %T", npi)
	}

	httpRestClient := client.ExtensionsV1beta1().RESTClient()
	uri := fmt.Sprintf("/apis/%s/%s/namespaces/%s/%s",
		strings.ToLower("extensions"),
		strings.ToLower("v1beta1"),
		strings.ToLower(namespace),
		strings.ToLower("NetworkPolicies"))

	result := httpRestClient.Post().RequestURI(uri).Body(np).Do()
	if result.Error() != nil {
		t.Fatal(result.Error())
	}
	defer func() {
		uri = fmt.Sprintf("/apis/%s/%s/namespaces/%s/%s/%s",
			strings.ToLower("extensions"),
			strings.ToLower("v1beta1"),
			strings.ToLower(namespace),
			strings.ToLower("NetworkPolicies"),
			strings.ToLower(np.ObjectMeta.Name))

		result = httpRestClient.Delete().RequestURI(uri).Do()
		if result.Error() != nil {
			t.Fatal(result.Error())
		}

	}()

	if err := wait.Poll(networkPingPollInterval, networkPingPollTimeout, func() (bool, error) {
		if err := nginx.IsUnReachable(); err != nil {
			t.Logf("error still reachable %s: %v", nginx.Name, err)
			return false, nil
		}
		return true, nil
	}); err != nil {
		t.Fatalf("default deny failed: %v", err)
	}
}

func HelperPolicy(t *testing.T, nginx *testworkload.Nginx) {
	netPolicy := fmt.Sprintf(string(netPolicyTpl), nginx.Name)
	npi, _, err := legacyscheme.Codecs.UniversalDecoder().Decode([]byte(netPolicy), nil, &v1beta1.NetworkPolicy{})
	if err != nil {
		t.Fatalf("unable to decode network policy manifest: %v", err)
	}
	np, ok := npi.(*v1beta1.NetworkPolicy)
	if !ok {
		t.Fatalf("expected manifest to decode into *api.networkpolicy, got %T", npi)
	}

	httpRestClient := client.ExtensionsV1beta1().RESTClient()
	uri := fmt.Sprintf("/apis/%s/%s/namespaces/%s/%s",
		strings.ToLower("extensions"),
		strings.ToLower("v1beta1"),
		strings.ToLower(namespace),
		strings.ToLower("NetworkPolicies"))

	result := httpRestClient.Post().RequestURI(uri).Body(np).Do()
	if result.Error() != nil {
		t.Fatal(result.Error())
	}
	defer func() {
		uri = fmt.Sprintf("/apis/%s/%s/namespaces/%s/%s/%s",
			strings.ToLower("extensions"),
			strings.ToLower("v1beta1"),
			strings.ToLower(namespace),
			strings.ToLower("NetworkPolicies"),
			strings.ToLower(np.ObjectMeta.Name))

		result = httpRestClient.Delete().RequestURI(uri).Do()
		if result.Error() != nil {
			t.Fatal(result.Error())
		}

	}()

	if err := wait.Poll(networkPingPollInterval, networkPingPollTimeout, func() (bool, error) {
		if err := nginx.IsReachable(); err != nil {
			t.Logf("error not reachable %s: %v", nginx.Name, err)
			return false, nil
		}
		return true, nil
	}); err != nil {
		t.Fatalf("allow nginx network policy failed: %v", err)
	}
}

var defaultDenyNetworkPolicy = []byte(`kind: NetworkPolicy
apiVersion: extensions/v1beta1
metadata:
  name: default-deny
spec:
  podSelector:
`)

var netPolicyTpl = []byte(`kind: NetworkPolicy
apiVersion: extensions/v1beta1
metadata:
  name: access-nginx
spec:
  podSelector:
    matchLabels:
      app: %s
  ingress:
    - from:
      - podSelector:
          matchLabels:
            allow: access
`)
