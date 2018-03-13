package asset

// DefaultImages are the defualt images bootkube components use.
var DefaultImages = ImageVersions{
	Etcd:            "quay.io/coreos/etcd:v3.1.8",
	Flannel:         "quay.io/coreos/flannel:v0.10.0-amd64",
	FlannelCNI:      "quay.io/coreos/flannel-cni:v0.3.0",
	Calico:          "quay.io/calico/node:v2.6.6",
	CalicoCNI:       "quay.io/calico/cni:v1.11.2",
	Hyperkube:       "gcr.io/google_containers/hyperkube:v1.9.4",
	KubeDNS:         "gcr.io/google_containers/k8s-dns-kube-dns-amd64:1.14.8",
	KubeDNSMasq:     "gcr.io/google_containers/k8s-dns-dnsmasq-nanny-amd64:1.14.8",
	KubeDNSSidecar:  "gcr.io/google_containers/k8s-dns-sidecar-amd64:1.14.8",
	PodCheckpointer: "quay.io/coreos/pod-checkpointer:3cd08279c564e95c8b42a0b97c073522d4a6b965",
}
