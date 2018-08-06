#!/usr/bin/env bash
set -euo pipefail

REMOTE_HOST=$1
KUBECONFIG=$2
REMOTE_PORT=${REMOTE_PORT:-22}
REMOTE_USER=${REMOTE_USER:-core}
IDENT=${IDENT:-${HOME}/.ssh/id_rsa}
SSH_OPTS=${SSH_OPTS:-}
TAG_MASTER=${TAG_MASTER:-false}
CLOUD_PROVIDER=${CLOUD_PROVIDER:-}

function usage() {
    echo "USAGE:"
    echo "$0: <remote-host> <kube-config>"
    exit 1
}

function retry_cmd() {
    set +e
    max_retries=$1; shift
    backoff=1
    retry=0
    false
    while [ $? -ne 0 ]; do
        if [ "$retry" -ge $max_retries ]; then
            break
        else
            retry=$((retry+1))
        fi
        "$@" || (sleep $((backoff *= 2)); false)
    done
    set -e
}

function wait_for_ssh() {
    retry_cmd 100 ssh -i ${IDENT} -p ${REMOTE_PORT} ${SSH_OPTS} ${REMOTE_USER}@${REMOTE_HOST} "uname -a"
}

# Initialize a worker node
function init_worker_node() {

    # Setup kubeconfig
    mkdir -p /etc/kubernetes
    cp ${KUBECONFIG} /etc/kubernetes/kubeconfig
    # Pulled out of the kubeconfig. Other installations should place the root
    # CA here manually.
    grep 'certificate-authority-data' ${KUBECONFIG} | awk '{print $2}' | base64 -d > /etc/kubernetes/ca.crt

    mv /home/${REMOTE_USER}/kubelet.service /etc/systemd/system/kubelet.service

    # Set cloud provider
    sed -i "s/cloud-provider=/cloud-provider=$CLOUD_PROVIDER/" /etc/systemd/system/kubelet.service

    if [ "$TAG_MASTER" = true ] ; then
        # Configure master label and taint
        echo -e 'node_label=node-role.kubernetes.io/master\nnode_taint=node-role.kubernetes.io/master=:NoSchedule' > /etc/kubernetes/kubelet.env
    fi

    # Start services
    systemctl daemon-reload
    systemctl stop locksmithd; systemctl mask locksmithd
    systemctl enable kubelet; sudo systemctl start kubelet
}

[ "$#" == 2 ] || usage

# This script can execute on a remote host by copying itself + kubelet service unit to remote host.
# After assets are available on the remote host, the script will execute itself in "local" mode.
if [ "${REMOTE_HOST}" != "local" ]; then

    # wait for ssh to be ready
    wait_for_ssh

    # Copy kubelet service file and kubeconfig to remote host
    scp -i ${IDENT} -P ${REMOTE_PORT} ${SSH_OPTS} kubelet.service ${REMOTE_USER}@${REMOTE_HOST}:/home/${REMOTE_USER}/kubelet.service
    scp -i ${IDENT} -P ${REMOTE_PORT} ${SSH_OPTS} ${KUBECONFIG} ${REMOTE_USER}@${REMOTE_HOST}:/home/${REMOTE_USER}/kubeconfig

    # Copy self to remote host so script can be executed in "local" mode
    scp -i ${IDENT} -P ${REMOTE_PORT} ${SSH_OPTS} ${BASH_SOURCE[0]} ${REMOTE_USER}@${REMOTE_HOST}:/home/${REMOTE_USER}/init-node.sh
    ssh -i ${IDENT} -p ${REMOTE_PORT} ${SSH_OPTS} ${REMOTE_USER}@${REMOTE_HOST} "sudo REMOTE_USER=${REMOTE_USER} TAG_MASTER=$TAG_MASTER CLOUD_PROVIDER=${CLOUD_PROVIDER} /home/${REMOTE_USER}/init-node.sh local /home/${REMOTE_USER}/kubeconfig"

    # Cleanup
    ssh -i ${IDENT} -p ${REMOTE_PORT} ${SSH_OPTS} ${REMOTE_USER}@${REMOTE_HOST} "rm /home/${REMOTE_USER}/{init-node.sh,kubeconfig}"

    echo
    echo "Node (${REMOTE_HOST}) bootstrap complete. It may take a few minutes for the node to become ready."
    if [ "$TAG_MASTER" = true ] ; then
        echo "NOTE: You have configured this node as a master. You may want to set up load balancing with your cloud provider so that masters receive equal traffic and gracefully fail over."
    fi

# Execute this script locally on the machine, assumes a kubelet.service file has already been placed on host.
elif [ "$1" == "local" ]; then
    init_worker_node
fi
