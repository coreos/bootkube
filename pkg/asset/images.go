package asset

// DefaultImages are the defualt images bootkube components use.
var DefaultImages = ImageVersions{
	Etcd:            "quay.io/coreos/etcd:v3.1.8",
	EtcdOperator:    "quay.io/coreos/etcd-operator:v0.5.2",
	Flannel:         "quay.io/coreos/flannel:v0.8.0-amd64",
	FlannelCNI:      "quay.io/coreos/flannel-cni:v0.3.0",
	Calico:          "quay.io/calico/node:v2.6.1",
	CalicoCNI:       "quay.io/calico/cni:v1.11.0",
	Hyperkube:       "quay.io/coreos/hyperkube:v1.8.1_coreos.0",
	Kenc:            "quay.io/coreos/kenc:0.0.2",
	KubeDNS:         "gcr.io/google_containers/k8s-dns-kube-dns-amd64:1.14.5",
	KubeDNSMasq:     "gcr.io/google_containers/k8s-dns-dnsmasq-nanny-amd64:1.14.5",
	KubeDNSSidecar:  "gcr.io/google_containers/k8s-dns-sidecar-amd64:1.14.5",
	PodCheckpointer: "quay.io/coreos/pod-checkpointer:ec22bec63334befacc2b237ab73b1a8b95b0a654",
}
