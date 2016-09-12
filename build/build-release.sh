#!/bin/bash
set -euo pipefail

BOOTKUBE_ROOT=$(git rev-parse --show-toplevel)
sudo rkt run \
    --volume bk,kind=host,source=${BOOTKUBE_ROOT} \
    --mount volume=bk,target=/go/src/github.com/kubernetes-incubator/bootkube \
    --insecure-options=image docker://golang:1.6.2 --exec /bin/bash -- -c \
    "cd /go/src/github.com/kubernetes-incubator/bootkube && make release"

# Default to building the bootkube image
BUILD_IMAGE=${BUILD_IMAGE:-bootkube}
source $BOOTKUBE_ROOT/build/build-image.sh
