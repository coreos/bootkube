#!/bin/bash
set -euo pipefail

# This is a generic image for building an image for any major command / project
# housed under this repo.
#
# To configure, set the `BUILD_IMAGE` environment variable, which will tell the script
# which image to build. Defaults to bootkube.
BUILD_IMAGE=${BUILD_IMAGE:-bootkube}

BOOTKUBE_ROOT=${BOOTKUBE_ROOT:-$(git rev-parse --show-toplevel)}

DOCKER_REPO=${DOCKER_REPO:-$(quay.io/coreos/${BUILD_IMAGE})}
DOCKER_TAG=${DOCKER_TAG:-$(${BOOTKUBE_ROOT}/build/git-version.sh)}
DOCKER_PUSH=${DOCKER_PUSH:-false}

TEMP_DIR=$(mktemp -d -t bootkube.$BUILD_IMAGE.XXXX)

cp $BOOTKUBE_ROOT/image/${BUILD_IMAGE}/* ${TEMP_DIR}
cp $BOOTKUBE_ROOT/_output/bin/linux/${BUILD_IMAGE} ${TEMP_DIR}/${BUILD_IMAGE}

docker build -t ${DOCKER_REPO}:${DOCKER_TAG} -f ${TEMP_DIR}/Dockerfile ${TEMP_DIR}
rm -rf ${TEMP_DIR}

if [[ ${DOCKER_PUSH} == "true" ]]; then
        docker push ${DOCKER_REPO}:${DOCKER_TAG}
fi
