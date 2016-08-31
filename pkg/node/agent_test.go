package node

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"k8s.io/kubernetes/pkg/api/v1"
)

func TestConfigHasChanged(t *testing.T) {
	testcases := []struct {
		onDisk   map[string]string
		desired  map[string]string
		expected bool
		desc     string
	}{
		{ // Nothing has changed.
			onDisk:   map[string]string{KubeletVersionKey: "version1"},
			desired:  map[string]string{KubeletVersionKey: "version1"},
			expected: false,
			desc:     "nothing changed",
		},
		{ // Version has changed.
			onDisk:   map[string]string{KubeletVersionKey: "version1"},
			desired:  map[string]string{KubeletVersionKey: "version2"},
			expected: true,
			desc:     "version changed",
		},
	}

	for _, tc := range testcases {
		actual := configHasChanged(tc.onDisk, tc.desired)
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
		desiredConfig        map[string]string
		expectedAnnotation   map[string]string
		expectedOnDiskConfig map[string]string
		shouldRestartUnit    bool
	}{
		{ // Nothing changed
			onDiskConfig:         map[string]string{KubeletVersionKey: "test1-version1"},
			expectedOnDiskConfig: map[string]string{KubeletVersionKey: "test1-version1"},
			expectedAnnotation:   map[string]string{KubeletVersionKey: "test1-version1"},
			shouldRestartUnit:    false,
		},
		{ // Version changed
			onDiskConfig:         map[string]string{KubeletVersionKey: "test2-version1"},
			desiredConfig:        map[string]string{KubeletVersionKey: "test2-version2"},
			expectedOnDiskConfig: map[string]string{KubeletVersionKey: "test2-version2"},
			expectedAnnotation:   map[string]string{KubeletVersionKey: "test2-version2"},
			shouldRestartUnit:    true,
		},
	}
	for _, tc := range testcases {
		var node v1.Node
		node.Annotations = make(map[string]string)
		conf, err := json.Marshal(tc.desiredConfig)
		if err != nil {
			t.Error(err)
		}
		node.Annotations[DesiredConfigAnnotation] = string(conf)

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

		updatedNode, err := a.handleConfigUpdate(node, tc.onDiskConfig, tc.desiredConfig, tmpf.Name())
		if err != nil {
			t.Fatal(err)
		}
		actualAnnotation := updatedNode.Annotations[CurrentConfigAnnotation]
		expectedAnnotationB, err := json.Marshal(tc.expectedAnnotation)
		if err != nil {
			t.Fatal(err)
		}
		expectedAnnotation := string(expectedAnnotationB)
		if actualAnnotation != expectedAnnotation {
			t.Fatalf("Node current annotation not correct, expected %#v, got %#v", expectedAnnotation, actualAnnotation)
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
