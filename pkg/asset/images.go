package asset

// DefaultImages are the defualt images bootkube components use.
var DefaultImages = ImageVersions{
	Etcd:            "quay.io/coreos/etcd:v3.1.8",
	Flannel:         "quay.io/coreos/flannel:v0.10.0-amd64",
	FlannelCNI:      "quay.io/coreos/flannel-cni:v0.3.0",
	Calico:          "quay.io/calico/node:v3.0.3",
	CalicoCNI:       "quay.io/calico/cni:v2.0.0",
	Hyperkube:       "k8s.gcr.io/hyperkube:v1.11.0",
	KubeDNS:         "k8s.gcr.io/k8s-dns-kube-dns-amd64:1.14.10",
	KubeDNSMasq:     "k8s.gcr.io/k8s-dns-dnsmasq-nanny-amd64:1.14.10",
	KubeDNSSidecar:  "k8s.gcr.io/k8s-dns-sidecar-amd64:1.14.10",
	PodCheckpointer: "quay.io/coreos/pod-checkpointer:9dc83e1ab3bc36ca25c9f7c18ddef1b91d4a0558",
}
