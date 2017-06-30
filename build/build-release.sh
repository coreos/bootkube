#!/usr/bin/env bash
set -euo pipefail

BOOTKUBE_ROOT=$(git rev-parse --show-toplevel)
sudo rkt run \
    --volume bk,kind=host,source=${BOOTKUBE_ROOT} \
    --mount volume=bk,target=/go/src/github.com/kubernetes-incubator/bootkube \
    --insecure-options=image docker://golang:1.8.3 --exec /bin/bash -- -c \
    "cd /go/src/github.com/kubernetes-incubator/bootkube && make release"
