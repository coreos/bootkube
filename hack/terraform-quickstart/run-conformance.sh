#!/usr/bin/env bash
set -euo pipefail

# This is a small shim to run the conformance runner script in
# /hack/tests/conformance-test.sh. More option defaults such as
# CONFORMANCE_VERSION are set in that script. If the SSH key you use to
# access the nodes setup by terraform is not available via the ssh agent then
# you must specify which keyfile to use.

export BOOTSTRAP_IP=`terraform output bootstrap_node_ip`
export KUBECONFIG=/etc/kubernetes/kubeconfig
export SSH_KEY_FILE=${SSH_KEY_FILE:-/fake/keyfile/have/agent}

cd ../tests
./conformance-test.sh $BOOTSTRAP_IP 22 ${SSH_KEY_FILE}

