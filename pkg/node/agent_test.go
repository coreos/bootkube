package node

import (
	"io/ioutil"
	"os"
	"testing"

	"k8s.io/kubernetes/pkg/api"

	"github.com/kubernetes-incubator/bootkube/pkg/cluster/components/version"
)

func TestConfigHasChanged(t *testing.T) {
	v1, err := version.ParseFromImageString("foo.io/bar/baz:v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	v2, err := version.ParseFromImageString("foo.io/bar/baz:v1.2.0")
	if err != nil {
		t.Fatal(err)
	}
	v3, err := version.ParseFromImageString("foo.io/bur/baz:v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	testcases := []struct {
		current  map[string]string
		desired  *version.Version
		expected bool
		desc     string
	}{
		{ // Nothing has changed.
			current:  map[string]string{KubeletImageKey: "foo.io/bar/baz", KubeletVersionKey: "v1.0.0"},
			desired:  v1,
			expected: false,
			desc:     "nothing changed",
		},
		{ // Version tag has changed.
			current:  map[string]string{KubeletImageKey: "foo.io/bar/baz", KubeletVersionKey: "v1.0.0"},
			desired:  v2,
			expected: true,
			desc:     "version changed",
		},
		{ // Version repo has changed.
			current:  map[string]string{KubeletImageKey: "foo.io/bar/baz", KubeletVersionKey: "v1.0.0"},
			desired:  v3,
			expected: true,
			desc:     "version changed",
		},
	}

	for _, tc := range testcases {
		actual := configHasChanged(tc.current, tc.desired)
		if actual != tc.expected {
			t.Errorf("expected %v got %v for test %s", tc.expected, actual, tc.desc)
		}
	}
}

type fakeDbusConn struct {
	reloaded      bool
	unitRestarted bool
}

func (f *fakeDbusConn) Reload() error {
	f.reloaded = true
	return nil
}

func (f *fakeDbusConn) RestartUnit(name string, action string, ch chan<- string) (int, error) {
	f.unitRestarted = true
	go func() { ch <- "done" }()
	return 0, nil
}

func TestHandleConfigUpdate(t *testing.T) {
	v1, err := version.ParseFromImageString("foo.io/bar/baz:v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	v2, err := version.ParseFromImageString("foo.io/bar/baz:v1.2.0")
	if err != nil {
		t.Fatal(err)
	}
	v3, err := version.ParseFromImageString("foo.io/bur/baz:v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	testcases := []struct {
		onDiskConfig         map[string]string
		desiredVersion       *version.Version
		expectedOnDiskConfig map[string]string
		shouldRestartUnit    bool
	}{
		{ // Nothing changed
			onDiskConfig:         map[string]string{KubeletImageKey: "foo.io/bar/baz", KubeletVersionKey: "v1.0.0"},
			expectedOnDiskConfig: map[string]string{KubeletImageKey: "foo.io/bar/baz", KubeletVersionKey: "v1.0.0"},
			desiredVersion:       v1,
			shouldRestartUnit:    false,
		},
		{ // Version tag changed
			onDiskConfig:         map[string]string{KubeletImageKey: "foo.io/bar/baz", KubeletVersionKey: "v1.0.0"},
			desiredVersion:       v2,
			expectedOnDiskConfig: map[string]string{KubeletImageKey: "foo.io/bar/baz", KubeletVersionKey: "v1.2.0"},
			shouldRestartUnit:    true,
		},
		{ // Version repo changed
			onDiskConfig:         map[string]string{KubeletImageKey: "foo.io/bar/baz", KubeletVersionKey: "v1.0.0"},
			desiredVersion:       v3,
			expectedOnDiskConfig: map[string]string{KubeletImageKey: "foo.io/bur/baz", KubeletVersionKey: "v1.0.0"},
			shouldRestartUnit:    true,
		},
	}
	for _, tc := range testcases {
		var node api.Node
		node.Annotations = make(map[string]string)
		node.Annotations[DesiredVersionAnnotation] = tc.desiredVersion.ImageString()

		tmpf, err := ioutil.TempFile("", "node-agent-test")
		if err != nil {
			t.Fatal(err)
		}
		defer tmpf.Close()
		defer os.Remove(tmpf.Name())

		fakeSysD := &fakeDbusConn{}
		a := &Agent{
			SysdConn: fakeSysD,
		}

		err = a.handleConfigUpdate(tc.onDiskConfig, tc.desiredVersion, tmpf.Name())
		if err != nil {
			t.Fatal(err)
		}
		actualOnDisk, err := parseKubeletEnvFile(tmpf.Name())
		if err != nil {
			t.Fatal(err)
		}
		if !mapMatch(actualOnDisk, tc.expectedOnDiskConfig) {
			t.Fatalf("on disk config not correct, expected %#v, got %#v", tc.expectedOnDiskConfig, actualOnDisk)
		}

		if tc.shouldRestartUnit {
			if !fakeSysD.reloaded || !fakeSysD.unitRestarted {
				t.Fatal("expected systemd unit to be restarted but was not")
			}
		}
	}
}

func mapMatch(m1, m2 map[string]string) bool {
	for k, v := range m1 {
		if m2[k] != v {
			return false
		}
	}
	return true
}
