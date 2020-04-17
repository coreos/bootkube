package helm

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kubernetes-sigs/bootkube/pkg/util"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/kube"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	defaultHelmStorageDriver = "secrets"
)

// InstallCharts installs all the helm charts in the given charts directory.
func InstallCharts(kubeconfigPath string, config clientcmd.ClientConfig, chartsDir string) error {
	// check if charts directory exists
	present, err := isExists(chartsDir)
	if err != nil {
		// error checking for the existence charts directory
		return err
	}
	// charts directory not found, nothing to do.
	if !present {
		return nil
	}
	// get all the charts
	namespaceChartsMap, err := getCharts(chartsDir)
	if err != nil {
		return fmt.Errorf("error getting charts from charts directory `%v`: %v", chartsDir, err)
	}
	// iterate over all the namespaces found in the charts directory
	for namespace, charts := range namespaceChartsMap {
		for _, chartName := range charts {
			chartPath := filepath.Join(chartsDir, namespace, chartName)
			// install charts found in each namespace directory
			if err := installChart(kubeconfigPath, namespace, chartName, chartPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// installChart is a helper function to install a single helm chart
func installChart(kubeconfigPath, namespace, chartName, chartPath string) error {
	client := kube.GetConfig(kubeconfigPath, "", namespace)
	actionConfig := &action.Configuration{}
	// namespace where helm stores the metadata about the releases
	helmStorageNamespace := getHelmStorageNamespace(namespace)
	if err := actionConfig.Init(client, helmStorageNamespace, defaultHelmStorageDriver, util.UserOutput); err != nil {
		util.UserOutput(fmt.Sprintf("error initalizing helm --- %v\n", err))
		return err
	}
	util.UserOutput(fmt.Sprintf("loading chart --- %s\n", chartName))
	// load chart from the directory
	chart, err := loader.Load(chartPath)
	if err != nil {
		return err
	}
	// set values to the default values of the chart
	values := chart.Values
	// values file is the same name as the chart name
	valuesFile := fmt.Sprintf("%s.yaml", chartPath)
	// if a valuesFile is provided use that as values
	valuesFileExists, err := isExists(valuesFile)
	if err != nil {
		// error checking for the existence values file
		return err
	}
	// load values file for a chart if values file exists
	if valuesFileExists {
		values, err = loadValuesFile(valuesFile)
		if err != nil {
			return err
		}
	}
	install := action.NewInstall(actionConfig)
	// Validate the chart and dependencies
	if err := validateChartAndDependencies(chart, install); err != nil {
		return err
	}
	install.ReleaseName = chartName
	install.Namespace = namespace
	release, err := install.Run(chart, values)
	if err != nil {
		return err
	}
	util.UserOutput(fmt.Sprintf("Release-created :: %s\n", release.Name))

	return nil
}

// isExists true true or false if the file/directory is present or not
func isExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err

	}

	return true, nil
}

// getHelmStorageNamespace gets the helm storage namespace from env variable HELM_STORAGE_NAMESPACE
func getHelmStorageNamespace(defaultStorageNamespace string) string {
	value, found := os.LookupEnv("HELM_STORAGE_NAMESPACE")
	if found || len(value) > 0 {
		return value
	}

	return defaultStorageNamespace
}

// validates the chart and its dependencies
func validateChartAndDependencies(chart *chart.Chart, client *action.Install) error {
	if err := chart.Validate(); err != nil {
		return err
	}
	// validate dependencies
	if req := chart.Metadata.Dependencies; req != nil {
		if err := action.CheckDependencies(chart, req); err != nil {
			if client.DependencyUpdate {
				man := &downloader.Manager{
					ChartPath:  chart.ChartFullPath(),
					Keyring:    client.ChartPathOptions.Keyring,
					SkipUpdate: false,
				}
				if err := man.Update(); err != nil {
					return err
				}
			} else {
				return err
			}
		}
	}

	return nil
}
