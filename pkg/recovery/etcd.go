// The etcd backend fetches control plane objects directly from etcd. This is adapted heavily from
// kubernetes/staging/src/k8s.io/apiserver/pkg/storage/etcd3/store.go.

package recovery

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/coreos/etcd/clientv3"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/pkg/api"
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
		decoder:    api.Codecs.UniversalDecoder(),
		pathPrefix: pathPrefix,
	}
}

// read implements Backend.read().
func (s *etcdBackend) read(ctx context.Context) (*controlPlane, error) {
	cp := &controlPlane{}
	for _, r := range []struct {
		etcdKeyName string
		yamlName    string
		obj         runtime.Object
	}{{
		etcdKeyName: "configmaps",
		yamlName:    "config-map",
		obj:         &cp.configMaps,
	}, {
		etcdKeyName: "daemonsets",
		yamlName:    "daemonset",
		obj:         &cp.daemonSets,
	}, {
		etcdKeyName: "deployments",
		yamlName:    "deployment",
		obj:         &cp.deployments,
	}, {
		etcdKeyName: "secrets",
		yamlName:    "secret",
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
	key = path.Join(s.pathPrefix, key, api.NamespaceSystem)
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

// list fetches a list runtime.Object from etcd located at key prefix `key`.
func (s *etcdBackend) list(ctx context.Context, key string, listObj runtime.Object) error {
	listPtr, err := meta.GetItemsPtr(listObj)
	if err != nil {
		return err
	}
	key = path.Join(s.pathPrefix, key, api.NamespaceSystem)
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
