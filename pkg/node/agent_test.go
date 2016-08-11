package node

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/client/cache"
)

func TestConfigHasChanged(t *testing.T) {
	testcases := []struct {
		cm       *v1.ConfigMap
		onDisk   map[string]string
		desired  map[string]string
		expected bool
		desc     string
	}{
		{ // Nothing has changed.
			cm:       &v1.ConfigMap{Data: map[string]string{configMapFlagsKey: "--api-servers='test'"}},
			onDisk:   map[string]string{kubeletVersionKey: "version1", kubeletFlagsKey: "--api-servers='test'"},
			desired:  map[string]string{kubeletVersionKey: "version1"},
			expected: false,
			desc:     "nothing changed",
		},
		{ // Version has changed.
			cm:       &v1.ConfigMap{Data: map[string]string{configMapFlagsKey: "--api-servers='test'"}},
			onDisk:   map[string]string{kubeletVersionKey: "version1", kubeletFlagsKey: "--api-servers='test'"},
			desired:  map[string]string{kubeletVersionKey: "version2"},
			expected: true,
			desc:     "version changed",
		},
		{ // ConfigMap flags have changed.
			cm:       &v1.ConfigMap{Data: map[string]string{configMapFlagsKey: "--api-servers='someting else'"}},
			onDisk:   map[string]string{kubeletVersionKey: "version1", kubeletFlagsKey: "--api-servers='test'"},
			desired:  map[string]string{kubeletVersionKey: "version1"},
			expected: true,
			desc:     "config map flags changed",
		},
	}

	for _, tc := range testcases {
		actual := configHasChanged(tc.onDisk, tc.desired, tc.cm)
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
		configMap            *v1.ConfigMap
		expectedAnnotation   map[string]string
		expectedOnDiskConfig map[string]string
		shouldRestartUnit    bool
	}{
		{ // Nothing changed
			onDiskConfig:         map[string]string{kubeletVersionKey: "test1-version1", kubeletFlagsKey: "--api-servers='test1'", "TEST_VAR": "TEST_VAL"},
			expectedOnDiskConfig: map[string]string{kubeletVersionKey: "test1-version1", kubeletFlagsKey: "--api-servers='test1'", "TEST_VAR": "TEST_VAL"},
			expectedAnnotation:   map[string]string{kubeletVersionKey: "test1-version1", kubeletFlagsKey: "--api-servers='test1'", kubeletConfigKey: "on-disk configuration"},
			shouldRestartUnit:    false,
		},
		{ // Version changed
			onDiskConfig:         map[string]string{kubeletVersionKey: "test2-version1", kubeletFlagsKey: "--api-servers='test2'", "TEST_VAR": "TEST_VAL"},
			desiredConfig:        map[string]string{kubeletVersionKey: "test2-version2", kubeletConfigKey: ""},
			expectedOnDiskConfig: map[string]string{kubeletVersionKey: "test2-version2", kubeletFlagsKey: "--api-servers='test2'", "TEST_VAR": "TEST_VAL"},
			expectedAnnotation:   map[string]string{kubeletVersionKey: "test2-version2", kubeletFlagsKey: "--api-servers='test2'", kubeletConfigKey: "on-disk configuration"},
			shouldRestartUnit:    true,
		},
		{ // Config changed
			onDiskConfig:         map[string]string{kubeletVersionKey: "test3-version1", kubeletFlagsKey: "--api-servers='test'", "TEST_VAR": "TEST_VAL"},
			desiredConfig:        map[string]string{kubeletVersionKey: "test3-version1", kubeletConfigKey: "updated-config"},
			configMap:            &v1.ConfigMap{ObjectMeta: v1.ObjectMeta{Name: "updated-config", Namespace: api.NamespaceSystem}, Data: map[string]string{configMapFlagsKey: "--api-servers='updated-test'"}},
			expectedOnDiskConfig: map[string]string{kubeletVersionKey: "test3-version1", kubeletFlagsKey: "--api-servers='updated-test'", "TEST_VAR": "TEST_VAL"},
			expectedAnnotation:   map[string]string{kubeletVersionKey: "test3-version1", kubeletFlagsKey: "--api-servers='updated-test'", kubeletConfigKey: "updated-config"},
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
		node.Annotations[desiredConfigAnnotation] = string(conf)

		store := cache.NewStore(cache.MetaNamespaceKeyFunc)
		if tc.configMap != nil {
			if err := store.Add(tc.configMap); err != nil {
				t.Fatal(err)
			}
		}

		tmpf, err := ioutil.TempFile("", "node-agent-test")
		if err != nil {
			t.Fatal(err)
		}
		defer tmpf.Close()
		defer os.Remove(tmpf.Name())

		fakeSysD := &fakeDbusConn{}
		a := &Agent{
			SysdConn:       fakeSysD,
			ConfigMapStore: store,
		}

		updatedNode, err := a.handleConfigUpdate(node, tc.onDiskConfig, tc.desiredConfig, tmpf.Name())
		if err != nil {
			t.Fatal(err)
		}
		actualAnnotation := updatedNode.Annotations[currentConfigAnnotation]
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
