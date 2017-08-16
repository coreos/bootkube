package asset

// DefaultImages are the defualt images bootkube components use.
var DefaultImages = ImageVersions{
	Etcd:            "quay.io/coreos/etcd:v3.1.8",
	EtcdOperator:    "quay.io/coreos/etcd-operator:v0.5.0",
	Flannel:         "quay.io/coreos/flannel:v0.8.0-amd64",
	FlannelCNI:      "quay.io/coreos/flannel-cni:0.1.0",
	Calico:          "quay.io/calico/node:v2.4.0",
	CalicoCNI:       "quay.io/calico/cni:v1.10.0",
	Hyperkube:       "quay.io/coreos/hyperkube:v1.7.1_coreos.0",
	Kenc:            "quay.io/coreos/kenc:0.0.2",
	KubeDNS:         "gcr.io/google_containers/k8s-dns-kube-dns-amd64:1.14.4",
	KubeDNSMasq:     "gcr.io/google_containers/k8s-dns-dnsmasq-nanny-amd64:1.14.4",
	KubeDNSSidecar:  "gcr.io/google_containers/k8s-dns-sidecar-amd64:1.14.4",
	PodCheckpointer: "quay.io/coreos/pod-checkpointer:4e7a7dab10bc4d895b66c21656291c6e0b017248",
}
