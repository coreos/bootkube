package bootkube

import (
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

	"github.com/coreos/bootkube/pkg/asset"
)

const (
	assetTimeout = 10 * time.Minute
	// NOTE: using 8081 as the port is a temporary hack when there is a single api-server.
	// The self-hosted apiserver will immediately die if it cannot bind to the insecure interface.
	// However, if it can successfully bind to insecure interface, it will continue to retry
	// failures on the the secure interface.
	// Staggering the insecure port allows us to launch a self-hosted api-server on the same machine
	// as the bootkube, and the self-hosted api-server will continually retry binding to secure interface
	// and doesn't end up in a race with bootkube for the insecure port. When bootkube dies, the self-hosted
	// api-server is using the correct standard ports (443/8080).
	insecureAPIAddr = "http://127.0.0.1:8080"
)

var requiredPods = []string{
	"kubelet",
	"kube-apiserver",
	"kube-scheduler",
	"kube-controller-manager",
}

type Config struct {
	AssetDir   string
	EtcdServer *url.URL
}

type bootkube struct {
	assetDir   string
	apiServer  *apiserver.APIServer
	controller *controller.CMServer
	scheduler  *scheduler.SchedulerServer
}

func NewBootkube(config Config) (*bootkube, error) {
	apiServer := apiserver.NewAPIServer()
	fs := pflag.NewFlagSet("apiserver", pflag.ExitOnError)
	apiServer.AddFlags(fs)
	fs.Parse([]string{
		"--bind-address=0.0.0.0",
		"--secure-port=443",
		"--insecure-port=8080",
		"--allow-privileged=true",
		"--tls-private-key-file=" + filepath.Join(config.AssetDir, asset.AssetPathAPIServerKey),
		"--tls-cert-file=" + filepath.Join(config.AssetDir, asset.AssetPathAPIServerCert),
		"--client-ca-file=" + filepath.Join(config.AssetDir, asset.AssetPathCACert),
		"--etcd-servers=" + config.EtcdServer.String(),
		"--service-cluster-ip-range=10.3.0.0/24",
		"--service-account-key-file=" + filepath.Join(config.AssetDir, asset.AssetPathServiceAccountPubKey),
		"--admission-control=ServiceAccount",
		"--runtime-config=extensions/v1beta1/deployments=true,extensions/v1beta1/daemonsets=true",
		"--token-auth-file=/etc/kubernetes/tokens",
	})

	cmServer := controller.NewCMServer()
	fs = pflag.NewFlagSet("controllermanager", pflag.ExitOnError)
	cmServer.AddFlags(fs)
	fs.Parse([]string{
		"--master=" + insecureAPIAddr,
		"--service-account-private-key-file=" + filepath.Join(config.AssetDir, asset.AssetPathServiceAccountPrivKey),
		"--root-ca-file=" + filepath.Join(config.AssetDir, asset.AssetPathCACert),
		"--allocate-node-cidrs=true",
		"--cluster-cidr=10.2.0.0/16",
		"--leader-elect=true",
		"--cluster-signing-cert-file=" + filepath.Join(config.AssetDir, asset.AssetPathCACert),
		"--cluster-signing-key-file=" + filepath.Join(config.AssetDir, asset.AssetPathCAKey),
	})

	schedServer := scheduler.NewSchedulerServer()
	fs = pflag.NewFlagSet("scheduler", pflag.ExitOnError)
	schedServer.AddFlags(fs)
	fs.Parse([]string{
		"--master=" + insecureAPIAddr,
		"--leader-elect=true",
	})

	return &bootkube{
		apiServer:  apiServer,
		controller: cmServer,
		scheduler:  schedServer,
		assetDir:   config.AssetDir,
	}, nil
}

func (b *bootkube) Run() error {
	errch := make(chan error)
	go func() { errch <- apiapp.Run(b.apiServer) }()
	go func() { errch <- cmapp.Run(b.controller) }()
	go func() { errch <- schedapp.Run(b.scheduler) }()
	go func() {
		if err := CreateAssets(filepath.Join(b.assetDir, asset.AssetPathManifests), assetTimeout); err != nil {
			errch <- err
		}
	}()
	go func() { errch <- WaitUntilPodsRunning(requiredPods, assetTimeout) }()

	// If any of the bootkube services exit, it means it is unrecoverable and we should exit.
	return <-errch
}
