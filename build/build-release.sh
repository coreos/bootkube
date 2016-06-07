#!/bin/bash
set -euo pipefail

BOOTKUBE_ROOT=$(git rev-parse --show-toplevel)
sudo rkt run \
    --volume bk,kind=host,source=${BOOTKUBE_ROOT} \
    --mount volume=bk,target=/go/src/github.com/coreos/bootkube \
    --insecure-options=image docker://golang:1.6.2 --exec /bin/bash -- -c \
    "cd /go/src/github.com/coreos/bootkube && make release"

source build-bootkube-image.sh
