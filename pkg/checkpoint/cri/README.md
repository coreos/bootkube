# Vendored CRI APIs

We copy generated CRI APIs for a couple reasons:
* When CRI promotes a version, it sometimes deletes the old alpha version.
* Prevents importing `k8s.io/kubernetes`.

The various versions of CRI are taken from:

* v1alpha1: https://github.com/kubernetes/kubernetes/tree/release-1.8/pkg/kubelet/apis/cri
* v1alpha1: https://github.com/kubernetes/kubernetes/tree/v1.9.6/pkg/kubelet/apis/cri
* v1alpha2: https://github.com/kubernetes/kubernetes/tree/v1.10.0-rc.1/pkg/kubelet/apis/cri
* v1alpha2: https://github.com/kubernetes/kubernetes/tree/release-1.11/pkg/kubelet/apis/cri
* v1alpha2: https://github.com/kubernetes/kubernetes/tree/release-1.12/pkg/kubelet/apis/cri
