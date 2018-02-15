// The etcd backend fetches control plane objects directly from etcd. This is adapted heavily from
// kubernetes/staging/src/k8s.io/apiserver/pkg/storage/etcd3/store.go.

package recovery

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/kubernetes-incubator/bootkube/pkg/asset"

	"github.com/coreos/etcd/clientv3"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
)

// etcdBackend is a backend that extracts a controlPlane from an etcd instance.
type etcdBackend struct {
	client     *clientv3.Client
	decoder    runtime.Decoder
	pathPrefix string
}

// NewEtcdBackend constructs a new etcdBackend for the given client and pathPrefix.
func NewEtcdBackend(client *clientv3.Client, pathPrefix string) Backend {
	return &etcdBackend{
		client:     client,
		decoder:    legacyscheme.Codecs.UniversalDecoder(),
		pathPrefix: pathPrefix,
	}
}

// read implements Backend.read().
func (s *etcdBackend) read(ctx context.Context) (*controlPlane, error) {
	cp := &controlPlane{}
	for _, r := range []struct {
		etcdKeyName string
		obj         runtime.Object
	}{{
		etcdKeyName: "configmaps",
		obj:         &cp.configMaps,
	}, {
		etcdKeyName: "daemonsets",
		obj:         &cp.daemonSets,
	}, {
		etcdKeyName: "deployments",
		obj:         &cp.deployments,
	}, {
		etcdKeyName: "secrets",
		obj:         &cp.secrets,
	}} {
		if err := s.list(ctx, r.etcdKeyName, r.obj); err != nil {
			return nil, err
		}
	}

	return cp, nil
}

// get fetches a single runtime.Object with key `key` from etcd.
func (s *etcdBackend) get(ctx context.Context, key string, out runtime.Object, ignoreNotFound bool) error {
	key = path.Join(s.pathPrefix, key, "kube-system")
	getResp, err := s.client.KV.Get(ctx, key)
	if err != nil {
		return err
	}

	if len(getResp.Kvs) == 0 {
		if ignoreNotFound {
			return runtime.SetZeroValue(out)
		}
		return fmt.Errorf("key not found: %s", key)
	}
	kv := getResp.Kvs[0]
	return decode(s.decoder, kv.Value, out)
}

func (s *etcdBackend) getBytes(ctx context.Context, key string) ([]byte, error) {
	key = path.Join(s.pathPrefix, key)
	getResp, err := s.client.KV.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if len(getResp.Kvs) == 0 {
		return nil, fmt.Errorf("key not found: %s", key)
	}
	return getResp.Kvs[0].Value, nil
}

// list fetches a list runtime.Object from etcd located at key prefix `key`.
func (s *etcdBackend) list(ctx context.Context, key string, listObj runtime.Object) error {
	listPtr, err := meta.GetItemsPtr(listObj)
	if err != nil {
		return err
	}
	key = path.Join(s.pathPrefix, key, "kube-system")
	if !strings.HasSuffix(key, "/") {
		key += "/"
	}
	getResp, err := s.client.KV.Get(ctx, key, clientv3.WithPrefix())
	if err != nil {
		return err
	}

	elems := make([][]byte, len(getResp.Kvs))
	for i, kv := range getResp.Kvs {
		elems[i] = kv.Value
	}
	return decodeList(elems, listPtr, s.decoder)
}

const (
	assetPathRecoveryEtcd  = "recovery-etcd.yaml"
	etcdCRDKey             = "etcd.database.coreos.com/etcdclusters/kube-system/kube-etcd"
	etcdMemberPodPrefix    = "pods/kube-system/kube-etcd-"
	RecoveryEtcdClientAddr = "http://localhost:52379"
)

// StartRecoveryEtcdForBackup starts a recovery etcd container using given backup.
// The started etcd server listens on RecoveryEtcdClientAddr.
func StartRecoveryEtcdForBackup(p, backupPath string) error {
	d, f := path.Split(backupPath)

	config := struct {
		Image      string
		BackupFile string
		BackupDir  string
		ClientAddr string
	}{
		Image:      asset.DefaultImages.Etcd,
		BackupFile: f,
		BackupDir:  d,
		ClientAddr: RecoveryEtcdClientAddr,
	}

	as := asset.MustCreateAssetFromTemplate(assetPathRecoveryEtcd, recoveryEtcdTemplate, config)
	return as.WriteFile(p)
}

// CleanRecoveryEtcd removes the recovery etcd static pod manifest and stops the recovery
// etcd container.
func CleanRecoveryEtcd(p string) error {
	return os.Remove(path.Join(p, assetPathRecoveryEtcd))
}

func stripPort(hostport string) string {
	colon := strings.IndexByte(hostport, ':')
	if colon == -1 {
		return hostport
	}
	if i := strings.IndexByte(hostport, ']'); i != -1 {
		return strings.TrimPrefix(hostport[:i], "[")
	}
	return hostport[:colon]
}
