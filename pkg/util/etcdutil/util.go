package etcdutil

import (
	"context"
	"crypto/tls"
	"path/filepath"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/pkg/transport"
	"github.com/golang/glog"
	"github.com/kubernetes-incubator/bootkube/pkg/asset"
	"k8s.io/apimachinery/pkg/util/wait"
)

// WaitClusterReady waits the etcd server ready to serve client requests.
func WaitClusterReady(endpoint string, etcdTLS *tls.Config) error {
	cfg := clientv3.Config{
		Endpoints:   []string{endpoint},
		TLS:         etcdTLS,
		DialTimeout: pollInterval,
	}
	err := wait.Poll(pollInterval, pollTimeout, func() (bool, error) {
		etcdcli, err := clientv3.New(cfg)
		if err != nil {
			glog.Infof("could not create etcd client, retrying later: %v", err)
			return false, nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), pollInterval)
		_, err = etcdcli.Get(ctx, "/")
		cancel()
		if err != nil {
			glog.Infof("could not read from etcd, retrying later: %v", err)
			return false, nil
		}
		return true, nil
	})

	return err
}

func makeTLSConfig(assetDir string) (*tls.Config, error) {
	tlsInfo := transport.TLSInfo{
		TrustedCAFile: filepath.Join(assetDir, asset.AssetPathEtcdCA),
		CertFile:      filepath.Join(assetDir, asset.AssetPathEtcdClientCert),
		KeyFile:       filepath.Join(assetDir, asset.AssetPathEtcdClientKey),
	}
	return tlsInfo.ClientConfig()
}
