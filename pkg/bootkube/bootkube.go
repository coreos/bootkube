package bootkube

import (
	"fmt"
	"net/url"
	"path/filepath"
	"time"

	"github.com/spf13/pflag"
	apiapp "k8s.io/kubernetes/cmd/kube-apiserver/app"
	apiserver "k8s.io/kubernetes/cmd/kube-apiserver/app/options"
	cmapp "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	controller "k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
	schedapp "k8s.io/kubernetes/plugin/cmd/kube-scheduler/app"
	scheduler "k8s.io/kubernetes/plugin/cmd/kube-scheduler/app/options"

	"github.com/kubernetes-incubator/bootkube/pkg/asset"
	"github.com/kubernetes-incubator/bootkube/pkg/util/etcdutil"
)

const (
	assetTimeout    = 20 * time.Minute
	insecureAPIAddr = "http://127.0.0.1:8080"
)

var requiredPods = []string{
	"pod-checkpointer",
	"kube-apiserver",
	"kube-scheduler",
	"kube-controller-manager",
}

type Config struct {
	AssetDir       string
	EtcdServer     *url.URL
	SelfHostedEtcd bool
}

type bootkube struct {
	selfHostedEtcd bool
	assetDir       string
	apiServer      *apiserver.ServerRunOptions
	controller     *controller.CMServer
	scheduler      *scheduler.SchedulerServer
}

func NewBootkube(config Config) (*bootkube, error) {
	apiServer := apiserver.NewServerRunOptions()
	fs := pflag.NewFlagSet("apiserver", pflag.ExitOnError)
	apiServer.AddFlags(fs)
	flags, err := makeAPIServerFlags(config)
	if err != nil {
		return nil, err
	}
	fs.Parse(flags)

	cmServer := controller.NewCMServer()
	fs = pflag.NewFlagSet("controllermanager", pflag.ExitOnError)
	cmServer.AddFlags(fs)
	flags, err = makeControllerManagerFlags(config)
	if err != nil {
		return nil, err
	}
	fs.Parse(flags)

	schedServer := scheduler.NewSchedulerServer()
	fs = pflag.NewFlagSet("scheduler", pflag.ExitOnError)
	schedServer.AddFlags(fs)
	fs.Parse([]string{
		"--master=" + insecureAPIAddr,
		"--leader-elect=true",
	})

	return &bootkube{
		apiServer:      apiServer,
		controller:     cmServer,
		scheduler:      schedServer,
		assetDir:       config.AssetDir,
		selfHostedEtcd: config.SelfHostedEtcd,
	}, nil
}

func makeAPIServerFlags(config Config) ([]string, error) {
	serviceCIDR, err := detectServiceCIDR(config)
	if err != nil {
		return []string{}, err
	}
	return []string{
		"--bind-address=0.0.0.0",
		"--secure-port=443",
		"--insecure-port=8080",
		"--allow-privileged=true",
		"--tls-private-key-file=" + filepath.Join(config.AssetDir, asset.AssetPathAPIServerKey),
		"--tls-cert-file=" + filepath.Join(config.AssetDir, asset.AssetPathAPIServerCert),
		"--client-ca-file=" + filepath.Join(config.AssetDir, asset.AssetPathCACert),
		"--etcd-servers=" + config.EtcdServer.String(),
		"--service-cluster-ip-range=" + serviceCIDR,
		"--service-account-key-file=" + filepath.Join(config.AssetDir, asset.AssetPathServiceAccountPubKey),
		"--admission-control=NamespaceLifecycle,ServiceAccount",
		"--runtime-config=api/all=true",
		"--storage-backend=etcd3",
	}, nil
}

func makeControllerManagerFlags(config Config) ([]string, error) {
	podCIDR, err := detectPodCIDR(config)
	if err != nil {
		return []string{}, err
	}
	return []string{
		"--master=" + insecureAPIAddr,
		"--service-account-private-key-file=" + filepath.Join(config.AssetDir, asset.AssetPathServiceAccountPrivKey),
		"--root-ca-file=" + filepath.Join(config.AssetDir, asset.AssetPathCACert),
		"--allocate-node-cidrs=true",
		"--cluster-cidr=" + podCIDR,
		"--configure-cloud-routes=false",
		"--leader-elect=true",
	}, nil
}

func (b *bootkube) Run() error {
	UserOutput("Running temporary bootstrap control plane...\n")

	errch := make(chan error)
	go func() { errch <- apiapp.Run(b.apiServer) }()
	go func() { errch <- cmapp.Run(b.controller) }()
	go func() { errch <- schedapp.Run(b.scheduler) }()
	go func() {
		if err := CreateAssets(filepath.Join(b.assetDir, asset.AssetPathManifests), assetTimeout); err != nil {
			errch <- err
		}
	}()
	go func() {
		if b.selfHostedEtcd {
			requiredPods = append(requiredPods, "etcd-operator")
			etcdServiceIP, err := detectEtcdIP(b.assetDir)
			if err != nil {
				errch <- err
				return
			}
			if err := WaitUntilPodsRunning(requiredPods, assetTimeout); err != nil {
				errch <- err
				return
			}
			errch <- etcdutil.Migrate(etcdServiceIP)
		} else {
			errch <- WaitUntilPodsRunning(requiredPods, assetTimeout)
		}
	}()
	// If any of the bootkube services exit, it means it is unrecoverable and we should exit.
	err := <-errch
	if err != nil {
		UserOutput("Error: %v\n", err)
	}
	return err
}

// All bootkube printing to stdout should go through this fmt.Printf wrapper.
// The stdout of bootkube should convey information useful to a human sitting
// at a terminal watching their cluster bootstrap itself. Otherwise the message
// should go to stderr.
func UserOutput(format string, a ...interface{}) {
	fmt.Printf(format, a...)
}
