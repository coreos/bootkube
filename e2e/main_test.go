package e2e

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/tools/clientcmd"
)

// global clients for use by all tests
var (
	client             kubernetes.Interface
	sshClient          *SSHClient
	expectedMasters    int // hint for tests to figure out how to fail or block on resources missing
	namespace          string
	enableExperimental = flag.Bool("enable-experimental", false, "If true, runs experimental/flaky tests.")
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
	namespace = fmt.Sprintf("bootkube-e2e-%x", rand.Int31())
}

// TestMain handles setup before all tests
func TestMain(m *testing.M) {
	var kubeconfig = flag.String("kubeconfig", "../hack/quickstart/cluster/auth/kubeconfig", "absolute path to the kubeconfig file")
	var keypath = flag.String("keypath", "", "path to private key for ssh client")
	flag.IntVar(&expectedMasters, "expectedmasters", 1, "hint needed for certain tests to fail, skip, or block on missing resources")
	var logOuputDir = flag.String("log-output-dir", "./logs", "logs from the tests will be populated in this directory")

	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	// create the clientset
	client, err = kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err := ready(client); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// create ssh client
	sshClient = newSSHClientOrDie(*keypath)

	// createNamespace
	if _, err := createNamespace(client, namespace); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// start log-collector
	// don't panic if it fails
	if err := startLogging(*keypath, *logOuputDir); err != nil {
		fmt.Println(err)
	}

	// run tests
	exitCode := m.Run()

	// cleanup log-collector assests
	// don't panic if it fails
	if err := cleanUpLogging(); err != nil {
		fmt.Println(err)
	}

	// Collect all logs
	// don't panic if it fails
	if err := collectLogs(); err != nil {
		fmt.Println(err)
	}

	if err := deleteNamespace(client, namespace); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	os.Exit(exitCode)
}

func createNamespace(c kubernetes.Interface, name string) (*v1.Namespace, error) {
	ns, err := c.CoreV1().Namespaces().Create(&v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	})
	if errors.IsAlreadyExists(err) {
		log.Println("ns already exists")
	} else if err != nil {
		return nil, fmt.Errorf("failed to create namespace with name %v %v", name, namespace)
	}

	return ns, nil
}

func deleteNamespace(c kubernetes.Interface, name string) error {
	return c.CoreV1().Namespaces().Delete(name, nil)
}

// Ready blocks until the cluster is considered available. The current
// implementation checks that 1 schedulable node is ready.
func ready(c kubernetes.Interface) error {
	return retry(50, 10*time.Second, func() error {
		list, err := c.CoreV1().Nodes().List(metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("error listing nodes: %v", err)
		}

		if len(list.Items) < 1 {
			return fmt.Errorf("cluster is not ready, waiting for 1 or more worker nodes, have %v", len(list.Items))
		}

		// check for 1 or more ready nodes by ignoring nodes marked unschedulable or containing taints
		for _, node := range list.Items {
			switch {
			case node.Spec.Unschedulable:
				log.Printf("worker node %q is unschedulable\n", node.Name)
			case len(node.Spec.Taints) != 0:
				log.Printf("worker node %q is tainted\n", node.Name)
			default:
				for _, condition := range node.Status.Conditions {
					if condition.Type == v1.NodeReady && condition.Status == v1.ConditionTrue {
						log.Printf("worker node %q is ready\n", node.Name)
						return nil
					}
				}
				log.Printf("worker node %q is not ready\n", node.Name)
			}
		}
		return fmt.Errorf("no worker nodes are ready, will retry")
	})
}

func retry(attempts int, delay time.Duration, f func() error) error {
	var err error

	for i := 0; i < attempts; i++ {
		err = f()
		if err == nil {
			break
		}

		if i < attempts-1 {
			glog.Errorf("attempt %d: %v", i, err)
			time.Sleep(delay)
		}
	}

	return err
}
