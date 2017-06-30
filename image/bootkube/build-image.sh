#!/usr/bin/env bash
set -euo pipefail

IMAGE_REPO=${IMAGE_REPO:-quay.io/coreos/bootkube}

readonly BOOTKUBE_ROOT=$(git rev-parse --show-toplevel)
readonly VERSION=${VERSION:-$(${BOOTKUBE_ROOT}/build/git-version.sh)}

function image::build() {
    local TEMP_DIR=$(mktemp -d -t bootkube.XXXX)

    cp $BOOTKUBE_ROOT/_output/bin/linux/bootkube ${TEMP_DIR}
    cp $BOOTKUBE_ROOT/image/bootkube/Dockerfile ${TEMP_DIR}

    docker build -t ${IMAGE_REPO}:${VERSION} -f ${TEMP_DIR}/Dockerfile ${TEMP_DIR}
    rm -rf ${TEMP_DIR}
}

function image::name() {
    echo "${IMAGE_REPO}:${VERSION}"
}

