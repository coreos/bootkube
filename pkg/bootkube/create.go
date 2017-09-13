package bootkube

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/kubernetes-incubator/bootkube/pkg/util"

	"github.com/golang/glog"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/tools/clientcmd"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
)

func CreateAssets(manifestDir string, timeout time.Duration) error {
	if _, err := os.Stat(manifestDir); os.IsNotExist(err) {
		UserOutput(fmt.Sprintf("WARNING: %v does not exist, not creating any self-hosted assets.\n", manifestDir))
		return nil
	}

	upFn := func() (bool, error) {
		if err := apiTest(); err != nil {
			glog.Warningf("Unable to determine api-server readiness: %v", err)
			return false, nil
		}
		return true, nil
	}

	UserOutput("Waiting for api-server...\n")
	if err := wait.Poll(5*time.Second, timeout, upFn); err != nil {
		err = fmt.Errorf("API Server is not ready: %v", err)
		glog.Error(err)
		return err
	}

	err := util.Retry(util.DefaultBackoff, func() error {
		UserOutput("Trying to create self-hosted assets...\n")
		return createAssets(manifestDir)
	})
	if err != nil {
		err = fmt.Errorf("Error creating assets: %v", err)
		glog.Error(err)
		UserOutput("%v\n", err)
		UserOutput("\nNOTE: Bootkube failed to create some cluster assets. It is important that manifest errors are resolved and resubmitted to the apiserver.\n")
		UserOutput("For example, after resolving issues: kubectl create -f <failed-manifest>\n\n")
	}

	return err
}

func createAssets(manifestDir string) error {
	f := cmdutil.NewFactory(kubeConfig)

	shouldValidate := true
	schema, err := f.Validator(shouldValidate, fmt.Sprintf("~/%s/%s", clientcmd.RecommendedHomeDir, clientcmd.RecommendedSchemaName))
	if err != nil {
		return err
	}

	cmdNamespace, enforceNamespace, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	mapper, _, err := f.UnstructuredObject()
	if err != nil {
		return err
	}

	builder, err := f.NewUnstructuredBuilder(true)
	if err != nil {
		return err
	}

	filenameOpts := &resource.FilenameOptions{
		Filenames: []string{manifestDir},
		Recursive: false,
	}

	r := builder.
		Schema(schema).
		ContinueOnError().
		NamespaceParam(cmdNamespace).DefaultNamespace().
		FilenameParam(enforceNamespace, filenameOpts).
		SelectorParam("").
		Flatten().
		Do()
	err = r.Err()
	if err != nil {
		return err
	}

	count := 0
	err = r.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}

		obj, err := resource.NewHelper(info.Client, info.Mapping).Create(info.Namespace, true, info.Object)
		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				count++
				return nil
			}
			return cmdutil.AddSourceToErr("creating", info.Source, err)
		}
		info.Refresh(obj, true)

		count++
		shortOutput := false
		if !shortOutput {
			f.PrintObjectSpecificMessage(info.Object, util.GlogWriter{})
		}
		cmdutil.PrintSuccess(mapper, shortOutput, util.GlogWriter{}, info.Mapping.Resource, info.Name, false, "created")
		UserOutput("\tcreated %23s %s\n", info.Name, strings.TrimSuffix(info.Mapping.Resource, "s"))
		return nil
	})
	if count == 0 {
		return fmt.Errorf("no objects passed to create")
	}
	return err
}

func apiTest() error {
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return err
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	// API Server is responding
	healthStatus := 0
	client.Discovery().RESTClient().Get().AbsPath("/healthz").Do().StatusCode(&healthStatus)
	if healthStatus != http.StatusOK {
		return fmt.Errorf("API Server http status: %d", healthStatus)
	}

	// System namespace has been created
	_, err = client.Namespaces().Get(api.NamespaceSystem, metav1.GetOptions{})
	return err
}
