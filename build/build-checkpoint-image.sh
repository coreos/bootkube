#!/bin/bash
set -euo pipefail

BOOTKUBE_ROOT=${BOOTKUBE_ROOT:-$(git rev-parse --show-toplevel)}

DOCKER_REPO=${DOCKER_REPO:-quay.io/coreos/pod-checkpointer}
DOCKER_TAG=${DOCKER_TAG:-$(${BOOTKUBE_ROOT}/build/git-version.sh)}
DOCKER_PUSH=${DOCKER_PUSH:-false}

TEMP_DIR=$(mktemp -d -t checkpoint.XXXX)

cp ../image/checkpoint/* ${TEMP_DIR}
cp ../_output/bin/linux/checkpoint ${TEMP_DIR}/checkpoint

docker build -t ${DOCKER_REPO}:${DOCKER_TAG} -f ${TEMP_DIR}/Dockerfile ${TEMP_DIR}
rm -rf ${TEMP_DIR}

if [[ ${DOCKER_PUSH} == "true" ]]; then
    docker push ${DOCKER_REPO}:${DOCKER_TAG}
fi
