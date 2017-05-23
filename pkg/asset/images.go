package asset

// DefaultImages are the defualt images bootkube components use.
var DefaultImages = ImageVersions{
	Busybox:         "busybox",
	Etcd:            "quay.io/coreos/etcd:v3.1.6",
	EtcdOperator:    "quay.io/coreos/etcd-operator:v0.3.0",
	Flannel:         "quay.io/coreos/flannel:v0.7.1-amd64",
	Hyperkube:       "quay.io/coreos/hyperkube:v1.6.4_coreos.0",
	Kenc:            "quay.io/coreos/kenc:48b6feceeee56c657ea9263f47b6ea091e8d3035",
	KubeDNS:         "gcr.io/google_containers/k8s-dns-kube-dns-amd64:1.14.1",
	KubeDNSMasq:     "gcr.io/google_containers/k8s-dns-dnsmasq-nanny-amd64:1.14.1",
	KubeDNSSidecar:  "gcr.io/google_containers/k8s-dns-sidecar-amd64:1.14.1",
	PodCheckpointer: "quay.io/coreos/pod-checkpointer:7da334a1d768c346798601eb8387f266d15cf330",
}
