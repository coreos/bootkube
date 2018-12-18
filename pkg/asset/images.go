package asset

// DefaultImages are the defualt images bootkube components use.
var DefaultImages = ImageVersions{
	Etcd:            "quay.io/coreos/etcd:v3.1.8",
	Flannel:         "quay.io/coreos/flannel:v0.10.0-amd64",
	FlannelCNI:      "quay.io/coreos/flannel-cni:v0.3.0",
	Calico:          "quay.io/calico/node:v3.0.3",
	CalicoCNI:       "quay.io/calico/cni:v2.0.0",
	CoreDNS:         "k8s.gcr.io/coredns:1.2.4",
	Hyperkube:       "k8s.gcr.io/hyperkube:v1.12.1",
	PodCheckpointer: "quay.io/coreos/pod-checkpointer:018007e77ccd61e8e59b7e15d7fc5e318a5a2682",
}
