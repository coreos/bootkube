#!/bin/bash
set -euo pipefail

BOOTKUBE_ROOT=${BOOTKUBE_ROOT:-$(git rev-parse --show-toplevel)}

DOCKER_REPO=${DOCKER_REPO:-quay.io/coreos/bootkube}
DOCKER_TAG=${DOCKER_TAG:-$(${BOOTKUBE_ROOT}/build/git-version.sh)}
DOCKER_PUSH=${DOCKER_PUSH:-false}

TEMP_DIR=$(mktemp -d -t bootkube.XXXX)

cp ../image/bootkube/* ${TEMP_DIR}
cp ../_output/bin/linux/bootkube ${TEMP_DIR}/bootkube

docker build -t ${DOCKER_REPO}:${DOCKER_TAG} -f ${TEMP_DIR}/Dockerfile ${TEMP_DIR}
rm -rf ${TEMP_DIR}

if [[ ${DOCKER_PUSH} == "true" ]]; then
    docker push ${DOCKER_REPO}:${DOCKER_TAG}
fi
