package bootkube

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang/glog"
	v1beta1ext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	"github.com/kubernetes-incubator/bootkube/pkg/util"
)

const (
	crdRolloutDuration = 5 * time.Second
	crdRolloutTimeout  = 2 * time.Minute
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

	createFn := func() (bool, error) {
		err := createAssets(manifestDir)
		if err != nil {
			err = fmt.Errorf("Error creating assets: %v", err)
			glog.Error(err)
			UserOutput("%v\n", err)
			UserOutput("\nNOTE: Bootkube failed to create some cluster assets. It is important that manifest errors are resolved and resubmitted to the apiserver.\n")
			UserOutput("For example, after resolving issues: kubectl create -f <failed-manifest>\n\n")
		}

		// Do not fail cluster creation due to missing assets as it is a recoverable situation
		// See https://github.com/kubernetes-incubator/bootkube/pull/368/files#r105509074
		return true, nil
	}

	UserOutput("Waiting for api-server...\n")
	start := time.Now()
	if err := wait.Poll(5*time.Second, timeout, upFn); err != nil {
		err = fmt.Errorf("API Server is not ready: %v", err)
		glog.Error(err)
		return err
	}

	UserOutput("Creating self-hosted assets...\n")
	timeout = timeout - time.Since(start)
	if err := wait.PollImmediate(5*time.Second, timeout, createFn); err != nil {
		err = fmt.Errorf("Failed to create assets: %v", err)
		glog.Error(err)
		return err
	}

	return nil
}

// TODO(derekparker) Although it may be difficult or tedious to move away from using the kubectl code
// we should consider refactoring this. The kubectl code tends to be very verbose and difficult to
// reason about, especially as the behevior of certain functions (e.g. `Visit`) can be difficult to
// predict with regards to error handling.
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
		gvk := obj.GetObjectKind().GroupVersionKind()

		// If the object is a CRD, wait for it to fully roll out before continuing.
		// This is because we may also be creating other instances of this CRD later.
		if gvk.Kind == "CustomResourceDefinition" {
			if err = waitForCRDRollout(info.Client, gvk, info.Object); err != nil {
				return err
			}
		}

		count++
		shortOutput := false
		if !shortOutput {
			f.PrintObjectSpecificMessage(info.Object, util.GlogWriter{})
		}
		cmdutil.PrintSuccess(mapper, shortOutput, util.GlogWriter{}, info.Mapping.Resource, info.Name, false, "created")
		UserOutput("\tcreated %23s %s\n", info.Name, strings.TrimSuffix(info.Mapping.Resource, "s"))
		return nil
	})
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("no objects passed to create")
	}
	return nil
}

func waitForCRDRollout(client resource.RESTClient, gvk schema.GroupVersionKind, obj runtime.Object) error {
	return wait.PollImmediate(crdRolloutDuration, crdRolloutTimeout, func() (bool, error) {
		var crd v1beta1ext.CustomResourceDefinition
		bytes, err := json.Marshal(obj)
		if err != nil {
			return false, err
		}
		if err = json.Unmarshal(bytes, &crd); err != nil {
			return false, err
		}
		uri := customResourceDefinitionKindURI(crd.Spec.Group, crd.Spec.Version, crd.GetNamespace(), crd.Spec.Names.Plural)
		res := client.Get().RequestURI(uri).Do()
		if res.Error() != nil {
			if apierrors.IsNotFound(res.Error()) {
				return false, nil
			}
			return false, res.Error()
		}
		return true, nil
	})
}

// customResourceDefinitionKindURI returns the URI for the CRD kind.
//
// Example of apiGroup: "tco.coreos.com"
// Example of version: "v1"
// Example of namespace: "default"
// Example of plural: "appversions"
func customResourceDefinitionKindURI(apiGroup, version, namespace, plural string) string {
	if namespace == "" {
		namespace = metav1.NamespaceDefault
	}
	return fmt.Sprintf("/apis/%s/%s/namespaces/%s/%s",
		strings.ToLower(apiGroup),
		strings.ToLower(version),
		strings.ToLower(namespace),
		strings.ToLower(plural))
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
	_, err = client.Namespaces().Get("kube-system", metav1.GetOptions{})
	return err
}
