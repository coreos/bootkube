#!/bin/bash
set -euo pipefail

BUILD_IMAGE=${BUILD_IMAGE:-}
PUSH_IMAGE=${PUSH_IMAGE:-false}
RELEASE=${RELEASE:-false}
MAKE_TARGET=${MAKE_TARGET:-all}

if [ -z "${BUILD_IMAGE}" ]; then
    echo "BUILD_IMAGE env var must be set"
    exit 1
fi

BOOTKUBE_ROOT=$(git rev-parse --show-toplevel)
if [[ "${RELEASE}" == "true" ]]; then
    source "${BOOTKUBE_ROOT}/build/build-release.sh"
else
  make ${MAKE_TARGET}
fi
source "${BOOTKUBE_ROOT}/image/${BUILD_IMAGE}/build-image.sh"

image::build
if [[ ${PUSH_IMAGE} == "true" ]]; then
    docker push $(image::name)
fi
