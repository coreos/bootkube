package asset

// DefaultImages are the defualt images bootkube components use.
var DefaultImages = ImageVersions{
	Etcd:            "quay.io/coreos/etcd:v3.1.8",
	EtcdOperator:    "quay.io/coreos/etcd-operator:v0.5.0",
	Flannel:         "quay.io/coreos/flannel:v0.9.1-amd64",
	FlannelCNI:      "quay.io/coreos/flannel-cni:v0.3.0",
	Calico:          "quay.io/calico/node:v2.6.4",
	CalicoCNI:       "quay.io/calico/cni:v1.11.1",
	Hyperkube:       "gcr.io/google_containers/hyperkube:v1.9.0",
	Kenc:            "quay.io/coreos/kenc:0.0.2",
	KubeDNS:         "gcr.io/google_containers/k8s-dns-kube-dns-amd64:1.14.5",
	KubeDNSMasq:     "gcr.io/google_containers/k8s-dns-dnsmasq-nanny-amd64:1.14.5",
	KubeDNSSidecar:  "gcr.io/google_containers/k8s-dns-sidecar-amd64:1.14.5",
	PodCheckpointer: "quay.io/coreos/pod-checkpointer:08fa021813231323e121ecca7383cc64c4afe888",
}
