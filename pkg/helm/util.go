package helm

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"sigs.k8s.io/yaml"
)

// getCharts returns the map structure of charts found in sub directories
// Sub directories of chartsDir corresponds to the namespaces the charts are to be installed.
// Each namespace sub directory contains the respectie charts.
// This method returns the map structure of namespace directory name as key and path of the charts as values.
func getCharts(chartsDir string) (map[string][]string, error) {
	var charts = map[string][]string{}
	dirs, err := getDirs(chartsDir)
	if err != nil {
		return nil, err
	}

	for _, dir := range dirs {
		chartsInDir := []string{}
		path := filepath.Join(chartsDir, dir)
		ch, err := getDirs(path)
		if err != nil {
			return nil, err
		}
		chartsInDir = append(chartsInDir, ch...)
		charts[dir] = chartsInDir
	}

	return charts, nil
}

// getDirs returns the list of directories in the given path.
func getDirs(path string) ([]string, error) {
	var dirs = []string{}
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return []string{}, err
	}

	for _, file := range files {
		if file.IsDir() {
			dirs = append(dirs, file.Name())
		}
	}

	return dirs, nil
}

// loadValuesFile reads the file from given path and returns the content in
// a map of key value pairs.
func loadValuesFile(path string) (map[string]interface{}, error) {
	values := map[string]interface{}{}
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(bytes, &values); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %v", path, err)
	}

	return values, nil
}
