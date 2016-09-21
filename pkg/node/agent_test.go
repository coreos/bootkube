package node

import (
	"io/ioutil"
	"os"
	"testing"

	"k8s.io/kubernetes/pkg/api/v1"
)

func TestConfigHasChanged(t *testing.T) {
	testcases := []struct {
		current  string
		desired  string
		expected bool
		desc     string
	}{
		{ // Nothing has changed.
			current:  "version1",
			desired:  "version1",
			expected: false,
			desc:     "nothing changed",
		},
		{ // Version has changed.
			current:  "version1",
			desired:  "version2",
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
	testcases := []struct {
		onDiskConfig         map[string]string
		desiredVersion       string
		expectedOnDiskConfig map[string]string
		shouldRestartUnit    bool
	}{
		{ // Nothing changed
			onDiskConfig:         map[string]string{KubeletVersionKey: "test1-version1"},
			expectedOnDiskConfig: map[string]string{KubeletVersionKey: "test1-version1"},
			desiredVersion:       "test1-version1",
			shouldRestartUnit:    false,
		},
		{ // Version changed
			onDiskConfig:         map[string]string{KubeletVersionKey: "test2-version1"},
			desiredVersion:       "test2-version2",
			expectedOnDiskConfig: map[string]string{KubeletVersionKey: "test2-version2"},
			shouldRestartUnit:    true,
		},
	}
	for _, tc := range testcases {
		var node v1.Node
		node.Annotations = make(map[string]string)
		node.Annotations[DesiredVersionAnnotation] = tc.desiredVersion

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
