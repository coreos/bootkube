package etcdutil

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/golang/glog"
)

func StartEtcd(endpoint string) error {
	if err := ioutil.WriteFile("/etc/kubernetes/manifests/boot-etcd.yaml", []byte(etcdPodYaml), 0600); err != nil {
		return fmt.Errorf("fail to write file '/etc/kubernetes/manifests/boot-etcd.yaml': %v", err)
	}
	glog.Info("etcd server has been defined to run by kubelet. Please wait...")
	return waitEtcdUp(endpoint)
}

func waitEtcdUp(endpoint string) error {
	httpcli := &http.Client{
		Timeout: 10 * time.Second,
	}
	for {
		_, err := httpcli.Get(endpoint + "/version")
		if err != nil {
			glog.Infof("couldn't talk to etcd server (retrying 10s later): %v\n", err)
			time.Sleep(10 * time.Second)
			continue
		}
		break
	}
	return nil
}

var etcdPodYaml = `apiVersion: v1
kind: Pod
metadata:
  name: boot-etcd
  namespace: kube-system
  labels:
    k8s-app: boot-etcd
spec:
  containers:
  - command:
    - /bin/sh
    - -c
    - /usr/local/bin/etcd
      --name boot-etcd
      --listen-client-urls=http://0.0.0.0:2379
      --listen-peer-urls=http://0.0.0.0:2380
      --advertise-client-urls=http://$(MY_POD_IP):2379
      --initial-advertise-peer-urls http://$(MY_POD_IP):2380
      --initial-cluster boot-etcd=http://$(MY_POD_IP):2380
      --initial-cluster-token bootkube
      --initial-cluster-state new
      --data-dir=/var/etcd/data
    env:
      - name: MY_POD_IP
        valueFrom:
          fieldRef:
            fieldPath: status.podIP
    image: quay.io/coreos/etcd:v3.1.0-alpha.1
    name: etcd
  hostNetwork: true
  restartPolicy: Never
`
