#!/bin/bash
set -euo pipefail

# DESCRIPTION:
#
# This script is meant to launch GCE nodes, run bootkube to bootstrap a self-hosted k8s cluster, then run conformance tests.
#
# REQUIREMENTS:
#  - gcloud cli is installed
#  - rkt is available on the host
#
# REQUIRED ENV VARS:
#  - $BUILD_ROOT: contains a checkout of bootkube at $BUILD_ROOT/bootkube
#  - $KEY_FILE:   path to GCE service account keyfile
#
# OPTIONAL ENV VARS:
#  - $BOOTKUBE_REPO:    container repo to use to launch bootkube. Default to value in quickstart/init-master.sh
#  - $BOOTKUBE_VERSION: container version to use to launch bootkube. Default to value in quickstart/init-master.sh
#  - $COREOS_VERSION:   CoreOS image version.
#
# PROCESS:
#
# Inside a rkt container:
#   - Use gcloud to launch master node
#     - Use the quickstart init-master.sh script to run bootkube on that node
#   - Use gcloud to launch worker node(s)
#     - Use the quickstart init-worker.sh script to join node to kubernetes cluster
#   - Run conformance tests against the launched cluster
#
BOOTKUBE_REPO=${BOOTKUBE_REPO:-}
BOOTKUBE_VERSION=${BOOTKUBE_VERSION:-}
COREOS_CHANNEL=${COREOS_CHANNEL:-'coreos-stable'}
WORKER_COUNT=4
GCE_PREFIX=${GCE_PREFIX:-'bootkube-ci'}

function cleanup {
    gcloud compute instances delete --quiet --zone us-central1-a ${GCE_PREFIX}-m1 || true
    gcloud compute firewall-rules delete --quiet ${GCE_PREFIX}-api-443 || true
    for i in $(seq 1 ${WORKER_COUNT}); do
        gcloud compute instances delete --quiet --zone us-central1-a ${GCE_PREFIX}-w${i} || true
    done
    rm -rf /build/cluster
}

function init {
    curl https://sdk.cloud.google.com | bash
    source ~/.bashrc
    gcloud config set project coreos-gce-testing
    gcloud auth activate-service-account ${GCE_PREFIX}@coreos-gce-testing.iam.gserviceaccount.com --key-file=/build/keyfile
    apt-get update && apt-get install -y jq

    ssh-keygen -t rsa -f /root/.ssh/id_rsa -N ""
    awk '{print "core:" $1 " " $2 " core@conformance"}' /root/.ssh/id_rsa.pub > /root/.ssh/gce-format.pub
}

function add_master {
    gcloud compute instances create ${GCE_PREFIX}-m1 \
        --image-project coreos-cloud --image-family ${COREOS_CHANNEL} --zone us-central1-a --machine-type n1-standard-4 --boot-disk-size=10GB

    gcloud compute instances add-tags --zone us-central1-a ${GCE_PREFIX}-m1 --tags ${GCE_PREFIX}-apiserver
    gcloud compute firewall-rules create ${GCE_PREFIX}-api-443 --target-tags=${GCE_PREFIX}-apiserver --allow tcp:443

    gcloud compute instances add-metadata ${GCE_PREFIX}-m1 --zone us-central1-a --metadata-from-file ssh-keys=/root/.ssh/gce-format.pub

    MASTER_IP=$(gcloud compute instances list ${GCE_PREFIX}-m1 --format=json | jq --raw-output '.[].networkInterfaces[].accessConfigs[].natIP')
    cd /build/bootkube/hack/quickstart && SSH_OPTS="-o StrictHostKeyChecking=no" \
        CLUSTER_DIR=/build/cluster BOOTKUBE_REPO=${BOOTKUBE_REPO} BOOTKUBE_VERSION=${BOOTKUBE_VERSION} ./init-master.sh ${MASTER_IP}
}

function add_workers {
    #TODO (aaron): parallelize launching workers
    for i in $(seq 1 ${WORKER_COUNT}); do
        gcloud compute instances create ${GCE_PREFIX}-w${i} \
            --image-project coreos-cloud --image-family ${COREOS_CHANNEL} --zone us-central1-a --machine-type n1-standard-1

        gcloud compute instances add-metadata ${GCE_PREFIX}-w${i} --zone us-central1-a --metadata-from-file ssh-keys=/root/.ssh/gce-format.pub

        local WORKER_IP=$(gcloud compute instances list ${GCE_PREFIX}-w${i} --format=json | jq --raw-output '.[].networkInterfaces[].accessConfigs[].natIP')
        cd /build/bootkube/hack/quickstart && SSH_OPTS="-o StrictHostKeyChecking=no" ./init-worker.sh ${WORKER_IP} /build/cluster/auth/kubeconfig
    done
}

IN_CONTAINER=${IN_CONTAINER:-false}
if [ "${IN_CONTAINER}" == true ]; then
    #TODO(aaron): should probably run cleanup as part of init (not just on exit). Or add some random identifier to objects created during this run.
    trap cleanup EXIT
    init
    add_master
    add_workers
    KUBECONFIG=/etc/kubernetes/kubeconfig WORKER_COUNT=${WORKER_COUNT} /build/bootkube/hack/tests/conformance-test.sh ${MASTER_IP} 22 /root/.ssh/id_rsa
else
    BUILD_ROOT=${BUILD_ROOT:-}
    if [ -z "$BUILD_ROOT" ]; then
        echo "BUILD_ROOT must be set"
        exit 1
    fi
    if [ -z "$KEY_FILE" ]; then
        echo "KEY_FILE must be set"
        exit 1
    fi

    RKT_OPTS=$(echo \
        "--volume buildroot,kind=host,source=${BUILD_ROOT} " \
        "--mount volume=buildroot,target=/build " \
        "--volume keyfile,kind=host,source=${KEY_FILE} " \
        "--mount volume=keyfile,target=/build/keyfile " \
    )

    sudo rkt run --insecure-options=image ${RKT_OPTS} docker://golang:1.7.4 --exec /bin/bash -- -c \
        "IN_CONTAINER=true BOOTKUBE_REPO=${BOOTKUBE_REPO} BOOTKUBE_VERSION=${BOOTKUBE_VERSION} COREOS_CHANNEL=${COREOS_CHANNEL} /build/bootkube/hack/tests/$(basename $0)"
fi
