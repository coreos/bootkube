#!/bin/bash
set -euo pipefail

CONFORMANCE_REPO=${CONFORMANCE_REPO:-github.com/coreos/kubernetes}
CONFORMANCE_VERSION=${CONFORMANCE_VERSION:-v1.6.2+coreos.1}

usage() {
    echo "USAGE:"
    echo "  $0 <ssh-host> <ssh-port> <ssh-key>"
    echo
    exit 1
}

if [ $# -ne 3 ]; then
    usage
    exit 1
fi

ssh_host=$1
ssh_port=$2
ssh_key=$3

KUBECONFIG=${KUBECONFIG:-/home/core/cluster/auth/kubeconfig}
K8S_SRC=/home/core/go/src/k8s.io/kubernetes
ssh -q -o stricthostkeychecking=no -i ${ssh_key} -p ${ssh_port} core@${ssh_host} \
    "mkdir -p ${K8S_SRC} && [[ -d ${K8S_SRC}/.git ]] || git clone https://${CONFORMANCE_REPO} ${K8S_SRC}"

RKT_OPTS="\
 --volume=kc,kind=host,source=${KUBECONFIG} \
 --volume=k8s,kind=host,source=${K8S_SRC} \
 --mount volume=kc,target=/kubeconfig \
 --mount volume=k8s,target=/go/src/k8s.io/kubernetes"

# Init steps necessary to run conformance in golang container
INIT="apt-get update && apt-get install -y rsync && go get -u github.com/jteeuwen/go-bindata/go-bindata"

BUILD="cd /go/src/k8s.io/kubernetes && \
 git checkout ${CONFORMANCE_VERSION} && \
 make all WHAT=cmd/kubectl && \
 make all WHAT=vendor/github.com/onsi/ginkgo/ginkgo && \
 make all WHAT=test/e2e/e2e.test"

CONFORMANCE="\
 KUBECONFIG=/kubeconfig KUBERNETES_PROVIDER=skeleton KUBERNETES_CONFORMANCE_TEST=Y go run hack/e2e.go \
 -v --test -check_version_skew=false --test_args='--ginkgo.focus=\[Conformance\]'"

CMD="sudo rkt run --insecure-options=image ${RKT_OPTS} docker://golang:1.7.5 --exec /bin/bash -- -c \"${INIT} && ${BUILD} && ${CONFORMANCE}\""
ssh -q -o stricthostkeychecking=no -i ${ssh_key} -p ${ssh_port} core@${ssh_host} "${CMD}"
