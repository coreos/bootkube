package bootkube

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	apierrors "k8s.io/kubernetes/pkg/api/errors"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/release_1_3"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	clientcmdapi "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/wait"
	"errors"
)

const (
	AssetToPrioritize = "kube-dns"
	AssetType = "service"
)

func CreateAssets(manifestDir string, timeout time.Duration) error {

	upFn := func() (bool, error) {
		if err := apiTest(); err != nil {
			glog.Warningf("Unable to determine api-server version: %v", err)
			return false, nil
		}
		return true, nil
	}

	createFn := func() (bool, error) {
		err := createAssets(manifestDir)
		if err != nil {
			glog.Errorf("Error creating assets: %v", err)
			// If error is "system namespace not found" we should retry
			if apierrors.IsNotFound(err) {
				details := err.(*apierrors.StatusError).Status().Details
				if details.Name == api.NamespaceSystem && details.Kind == "namespaces" {
					return false, nil // retry
				}
			}
			UserOutput("\nError creating assets: %v\n", err)
			UserOutput("\nNOTE: Bootkube failed to create some cluster assets. It is important that manifest errors are resolved and resubmitted to the apiserver.\n")
			UserOutput("For example, after resolving issues: kubectl create -f <failed-manifest>\n\n")
		}
		return true, nil
	}

	UserOutput("Waiting for api-server...\n")
	start := time.Now()
	if err := wait.Poll(5*time.Second, timeout, upFn); err != nil {
		return fmt.Errorf("API Server unavailable: %v", err)
	}

	UserOutput("Creating self-hosted assets...\n")
	timeout = timeout - time.Since(start)
	if err := wait.Poll(5*time.Second, timeout, createFn); err != nil {
		return fmt.Errorf("Failed to create assets: %v", err)
	}

	return nil
}

func createAssets(manifestDir string) error {
	config := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{},
		&clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: insecureAPIAddr}},
	)
	f := cmdutil.NewFactory(config)

	shouldValidate := true
	schema, err := f.Validator(shouldValidate, fmt.Sprintf("~/%s/%s", clientcmd.RecommendedHomeDir, clientcmd.RecommendedSchemaName))
	if err != nil {
		return err
	}

	cmdNamespace, enforceNamespace, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	includeExtendedAPIs := false
	mapper, typer := f.Object(includeExtendedAPIs)

	recursive := false
	r := resource.NewBuilder(mapper, typer, resource.ClientMapperFunc(f.ClientForMapping), f.Decoder(true)).
		Schema(schema).
		ContinueOnError().
		NamespaceParam(cmdNamespace).DefaultNamespace().
		FilenameParam(enforceNamespace, recursive, manifestDir).
		Flatten().
		Do()
	err = r.Err()
	if err != nil {
		return err
	}
	count := 0
	i, err := r.Infos()
	assets, e := prioritizeAssetCreation(i, AssetToPrioritize, AssetType)
	if e != nil {
		glog.Warningf("Asset ordering failed due to: %v", e)
	}
	for _, a := range assets {
		obj, err := resource.NewHelper(a.Client, a.Mapping).Create(a.Namespace, true, a.Object)
		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				return nil
			}
			return cmdutil.AddSourceToErr("creating", a.Source, err)
		}
		a.Refresh(obj, true)

		count++
		shortOutput := false
		if !shortOutput {
			f.PrintObjectSpecificMessage(a.Object, util.GlogWriter{})
		}
		cmdutil.PrintSuccess(mapper, shortOutput, util.GlogWriter{}, a.Mapping.Resource, a.Name, "created")
		UserOutput("\tcreated %23s %s\n", a.Name, strings.TrimSuffix(a.Mapping.Resource, "s"))

	}
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("no objects passed to create")
	}
	return nil
}

// To prevent another service from winning the race for dns service IP allocation
// we bump the dns service to the top of the list and create the DNS service asset first.
func prioritizeAssetCreation(origSlice []*resource.Info, assetToPrioritize, assetType string) ([]*resource.Info, error) {
	var seen bool = false
	sortedSlice := make([]*resource.Info, 1)
	for _, info := range origSlice {
		if info.Name == assetToPrioritize && strings.TrimSuffix(info.Mapping.Resource, "s") == assetType {
			sortedSlice[0] = info
			seen = true
		} else {
			sortedSlice = append(sortedSlice, info)
		}
	}
	if seen {
		return sortedSlice, nil
	}
	// if asset def. is not found return orig. slice and fail silently
	return origSlice, errors.New("Asset not found " + assetToPrioritize)
}

func apiTest() error {
	client, err := clientset.NewForConfig(&restclient.Config{Host: insecureAPIAddr})
	if err != nil {
		return err
	}

	_, err = client.Discovery().ServerVersion()
	return err
}
