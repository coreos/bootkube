#!/bin/bash
set -euo pipefail

MIRROR=${MIRROR:-quay.io}
IMAGE_REPO=${IMAGE_REPO:-${MIRROR}/coreos/pod-checkpointer}

readonly BOOTKUBE_ROOT=$(git rev-parse --show-toplevel)
readonly VERSION=${VERSION:-$(${BOOTKUBE_ROOT}/build/git-version.sh)}

function image::build() {
    local TEMP_DIR=$(mktemp -d -t checkpoint.XXXX)

    # Add assets for container build
    cp ${BOOTKUBE_ROOT}/_output/bin/linux/checkpoint ${TEMP_DIR}
    cp ${BOOTKUBE_ROOT}/image/checkpoint/Dockerfile ${TEMP_DIR}
    cp ${BOOTKUBE_ROOT}/image/checkpoint/checkpoint-install.sh ${TEMP_DIR}
    cp ${BOOTKUBE_ROOT}/image/checkpoint/checkpoint-pod.yaml ${TEMP_DIR}
    sed -i "s#{{ REPO }}:{{ TAG }}#${IMAGE_REPO}:${VERSION}#" ${TEMP_DIR}/checkpoint-pod.yaml
    sed -i "s#{{ MIRROR }}#${MIRROR}#" ${TEMP_DIR}/checkpoint-install.sh

    docker build -t ${IMAGE_REPO}:${VERSION} -f ${TEMP_DIR}/Dockerfile ${TEMP_DIR}
    rm -rf ${TEMP_DIR}
}

function image::name() {
    echo "${IMAGE_REPO}:${VERSION}"
}
