#!/bin/bash
set -euo pipefail

REMOTE_HOST=$1
KUBECONFIG=$2
REMOTE_PORT=${REMOTE_PORT:-22}
IDENT=${IDENT:-${HOME}/.ssh/id_rsa}
SSH_OPTS=${SSH_OPTS:-}

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

    mv /home/core/kubelet.worker /etc/systemd/system/kubelet.service

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
    scp -i ${IDENT} -P ${REMOTE_PORT} ${SSH_OPTS} kubelet.worker core@${REMOTE_HOST}:/home/core/kubelet.worker
    scp -i ${IDENT} -P ${REMOTE_PORT} ${SSH_OPTS} ${KUBECONFIG} core@${REMOTE_HOST}:/home/core/kubeconfig

    # Copy self to remote host so script can be executed in "local" mode
    scp -i ${IDENT} -P ${REMOTE_PORT} ${SSH_OPTS} ${BASH_SOURCE[0]} core@${REMOTE_HOST}:/home/core/init-worker.sh
    ssh -i ${IDENT} -p ${REMOTE_PORT} ${SSH_OPTS} core@${REMOTE_HOST} "sudo /home/core/init-worker.sh local /home/core/kubeconfig"

    # Cleanup
    ssh -i ${IDENT} -p ${REMOTE_PORT} ${SSH_OPTS} core@${REMOTE_HOST} "rm /home/core/init-worker.sh"

    echo
    echo "Node bootstrap complete. It may take a few minutes for the node to become ready. Access your kubernetes cluster using:"
    echo "kubectl --kubeconfig=${KUBECONFIG} get nodes"
    echo

# Execute this script locally on the machine, assumes a kubelet.service file has already been placed on host.
elif [ "$1" == "local" ]; then
    init_worker_node
fi
