package helm

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func setUp(t *testing.T) (string, []string, []string) {
	// Create charts directory structure
	var err error
	namespaceDirs := make([]string, 0)
	chartDirs := make([]string, 0)
	chartsDir, err := ioutil.TempDir("", "charts")
	if err != nil {
		t.Fatal(err)
	}
	kubesystemDir, err := ioutil.TempDir(chartsDir, "kube-system")
	if err != nil {
		t.Fatal(err)
	}
	calicoDir, err := ioutil.TempDir(kubesystemDir, "calico")
	if err != nil {
		t.Fatal(err)
	}
	flannelDir, err := ioutil.TempDir(kubesystemDir, "flannel")
	if err != nil {
		t.Fatal(err)
	}

	defaultDir, err := ioutil.TempDir(chartsDir, "default")
	if err != nil {
		t.Fatal(err)
	}

	someChartdir, err := ioutil.TempDir(defaultDir, "somechart")
	if err != nil {
		t.Fatal(err)
	}

	namespaceDirs = append(namespaceDirs, kubesystemDir)
	namespaceDirs = append(namespaceDirs, defaultDir)

	chartDirs = append(chartDirs, calicoDir)
	chartDirs = append(chartDirs, flannelDir)
	chartDirs = append(chartDirs, someChartdir)

	return chartsDir, namespaceDirs, chartDirs
}

func tearDown(chartsDir string, t *testing.T) {
	if err := os.RemoveAll(chartsDir); err != nil {
		t.Fatal(err)
	}
}

func TestGetCharts(t *testing.T) {
	chartsDir, namespaceDirs, chartDirs := setUp(t)
	defer tearDown(chartsDir, t)

	namespaceChartsMap, err := getCharts(chartsDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(namespaceDirs) != len(namespaceChartsMap) {
		t.Fatal("number of directories did not match")
	}

	for namespace, charts := range namespaceChartsMap {
		path := filepath.Join(chartsDir, namespace)
		if !isPresent(path, namespaceDirs) {
			t.Fatalf("did not find namespace directory named `%s`", namespace)
		}

		for _, chart := range charts {
			path = filepath.Join(chartsDir, namespace, chart)
			if !isPresent(path, chartDirs) {
				t.Fatalf("chart `%s` not found", chart)
			}

		}
	}
}

func isPresent(find string, from []string) bool {
	for _, entries := range from {
		if entries == find {
			return true
		}
	}

	return false
}

func TestLoadValuesFile(t *testing.T) {
	values, err := ioutil.TempDir("", "values")
	if err != nil {
		t.Fatal(err)
	}
	content := "key: value"
	valuesFile := filepath.Join(values, "values.yaml")
	err = ioutil.WriteFile(valuesFile, []byte(content), os.FileMode(0644))
	if err != nil {
		t.Fatal(err)
	}
	data, err := loadValuesFile(valuesFile)
	if err != nil {
		t.Fatal(err)
	}
	if data["key"] != "value" {
		t.Fatal("data not loaded properly")
	}
}
