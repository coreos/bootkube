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

    # Start services
    systemctl daemon-reload
    systemctl stop update-engine; systemctl mask update-engine
    systemctl enable kubelet; sudo systemctl start kubelet
}

[ "$#" == 2 ] || usage

# This script can execute on a remote host by copying itself + kubelet service unit to remote host.
# After assets are available on the remote host, the script will execute itself in "local" mode.
if [ "${REMOTE_HOST}" != "local" ]; then

    # Copy kubelet service file and kubeconfig to remote host
    if [ "$TAG_MASTER" = true ] ; then
        scp -i ${IDENT} -P ${REMOTE_PORT} ${SSH_OPTS} kubelet.master ${REMOTE_USER}@${REMOTE_HOST}:/home/${REMOTE_USER}/kubelet.service
    else
        scp -i ${IDENT} -P ${REMOTE_PORT} ${SSH_OPTS} kubelet.worker ${REMOTE_USER}@${REMOTE_HOST}:/home/${REMOTE_USER}/kubelet.service
    fi
    scp -i ${IDENT} -P ${REMOTE_PORT} ${SSH_OPTS} ${KUBECONFIG} ${REMOTE_USER}@${REMOTE_HOST}:/home/${REMOTE_USER}/kubeconfig

    # Copy self to remote host so script can be executed in "local" mode
    scp -i ${IDENT} -P ${REMOTE_PORT} ${SSH_OPTS} ${BASH_SOURCE[0]} ${REMOTE_USER}@${REMOTE_HOST}:/home/${REMOTE_USER}/init-node.sh
    ssh -i ${IDENT} -p ${REMOTE_PORT} ${SSH_OPTS} ${REMOTE_USER}@${REMOTE_HOST} "sudo REMOTE_USER=${REMOTE_USER} CLOUD_PROVIDER=${CLOUD_PROVIDER} /home/${REMOTE_USER}/init-node.sh local /home/${REMOTE_USER}/kubeconfig"

    # Cleanup
    ssh -i ${IDENT} -p ${REMOTE_PORT} ${SSH_OPTS} ${REMOTE_USER}@${REMOTE_HOST} "rm /home/${REMOTE_USER}/init-node.sh"

    echo
    echo "Node bootstrap complete. It may take a few minutes for the node to become ready. Access your kubernetes cluster using:"
    echo "kubectl --kubeconfig=${KUBECONFIG} get nodes"
    echo

# Execute this script locally on the machine, assumes a kubelet.service file has already been placed on host.
elif [ "$1" == "local" ]; then
    init_worker_node
fi
