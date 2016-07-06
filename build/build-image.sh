#!/bin/bash
set -euo pipefail

# This is a generic image for building an image for any major command / project
# housed under this repo.
#
# To configure, set the `CMD` environment variable, which will tell the script
# which image to build. Defaults to bootkube.
CMD=${CMD:-bootkube}

BOOTKUBE_ROOT=${BOOTKUBE_ROOT:-$(git rev-parse --show-toplevel)}

DOCKER_REPO=${DOCKER_REPO:-quay.io/coreos/$CMD}
DOCKER_TAG=${DOCKER_TAG:-$(${BOOTKUBE_ROOT}/build/git-version.sh)}
DOCKER_PUSH=${DOCKER_PUSH:-false}

TEMP_DIR=$(mktemp -d -t bootkube.$CMD.XXXX)

cp $BOOTKUBE_ROOT/image/$CMD/* ${TEMP_DIR}
cp $BOOTKUBE_ROOT/_output/bin/linux/$CMD ${TEMP_DIR}/$CMD

docker build -t ${DOCKER_REPO}:${DOCKER_TAG} -f ${TEMP_DIR}/Dockerfile ${TEMP_DIR}
rm -rf ${TEMP_DIR}

if [[ ${DOCKER_PUSH} == "true" ]]; then
    docker push ${DOCKER_REPO}:${DOCKER_TAG}
fi
