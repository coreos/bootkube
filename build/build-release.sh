#!/usr/bin/env bash
set -euo pipefail

BOOTKUBE_ROOT=$(git rev-parse --show-toplevel)
sudo rkt run \
    --volume bk,kind=host,source=${BOOTKUBE_ROOT} \
    --mount volume=bk,target=/go/src/github.com/kubernetes-sigs/bootkube \
    --insecure-options=image docker://golang:1.13.0 --exec /bin/bash -- -c \
    "cd /go/src/github.com/kubernetes-sigs/bootkube && make release"
