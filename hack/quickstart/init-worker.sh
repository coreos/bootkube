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

function extract_master_endpoint (){
    grep 'certificate-authority-data' ${KUBECONFIG} | awk '{print $2}' | base64 -d > /home/core/ca.crt
    grep 'client-certificate-data' ${KUBECONFIG} | awk '{print $2}'| base64 -d > /home/core/client.crt
    grep 'client-key-data' ${KUBECONFIG} | awk '{print $2}'| base64 -d > /home/core/client.key

    MASTER_PUB="$(awk '/server:/ {print $2}' ${KUBECONFIG} | awk -F/ '{print $3}' | awk -F: '{print $1}')"
    # TODO (aaron): The -k was added with the gce conformance tests - figure out why it's needed here.
    #     The certs are seemingly signed correctly, but says no SAN for MASTER_PUB
    MASTER_PRIV=$(curl -k https://${MASTER_PUB}:443/api/v1/namespaces/default/endpoints/kubernetes \
        --cacert /home/core/ca.crt --cert /home/core/client.crt --key /home/core/client.key \
        | jq -r '.subsets[0].addresses[0].ip')
    rm -f /home/core/ca.crt /home/core/client.crt /home/core/client.key
}

# Initialize a worker node
function init_worker_node() {
    extract_master_endpoint

    # Setup kubeconfig
    mkdir -p /etc/kubernetes
    cp ${KUBECONFIG} /etc/kubernetes/kubeconfig

    sed "s/{{apiserver}}/${MASTER_PRIV}/" /home/core/kubelet.worker > /etc/systemd/system/kubelet.service
    rm /home/core/kubelet.worker

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
