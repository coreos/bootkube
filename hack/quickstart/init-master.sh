#!/bin/bash
set -euo pipefail

REMOTE_HOST=$1
REMOTE_PORT=${REMOTE_PORT:-22}
CLUSTER_DIR=${CLUSTER_DIR:-cluster}
IDENT=${IDENT:-${HOME}/.ssh/id_rsa}
SSH_OPTS=${SSH_OPTS:-}

BOOTKUBE_REPO=${BOOTKUBE_REPO:-quay.io/coreos/bootkube}
BOOTKUBE_VERSION=${BOOTKUBE_VERSION:-v0.3.0}

function usage() {
    echo "USAGE:"
    echo "$0: <remote-host>"
    exit 1
}

function configure_etcd() {
    [ -f "/etc/systemd/system/etcd2.service.d/10-etcd2.conf" ] || {
        mkdir -p /etc/systemd/system/etcd2.service.d
        cat << EOF > /etc/systemd/system/etcd2.service.d/10-etcd2.conf
[Service]
Environment="ETCD_NAME=controller"
Environment="ETCD_INITIAL_CLUSTER=controller=http://${COREOS_PRIVATE_IPV4}:2380"
Environment="ETCD_INITIAL_ADVERTISE_PEER_URLS=http://${COREOS_PRIVATE_IPV4}:2380"
Environment="ETCD_ADVERTISE_CLIENT_URLS=http://${COREOS_PRIVATE_IPV4}:2379"
Environment="ETCD_LISTEN_CLIENT_URLS=http://0.0.0.0:2379"
Environment="ETCD_LISTEN_PEER_URLS=http://0.0.0.0:2380"
EOF
    }
}

# Initialize a Master node
function init_master_node() {
    systemctl daemon-reload
    systemctl stop update-engine; systemctl mask update-engine

    # Start etcd and configure network settings
    configure_etcd
    systemctl enable etcd2; sudo systemctl start etcd2

    # Render cluster assets
    /usr/bin/rkt run \
        --volume home,kind=host,source=/home/core \
        --mount volume=home,target=/core \
        --trust-keys-from-https --net=host ${BOOTKUBE_REPO}:${BOOTKUBE_VERSION} --exec \
        /bootkube -- render --asset-dir=/core/assets --api-servers=https://${COREOS_PUBLIC_IPV4}:443,https://${COREOS_PRIVATE_IPV4}:443

    # Move the local kubeconfig into expected location
    chown -R core:core /home/core/assets
    mkdir -p /etc/kubernetes
    cp /home/core/assets/auth/kubeconfig /etc/kubernetes/

    # Start the kubelet
    systemctl enable kubelet; sudo systemctl start kubelet

    # Start bootkube to launch a self-hosted cluster
    /usr/bin/rkt run \
        --volume home,kind=host,source=/home/core \
        --mount volume=home,target=/core \
        --net=host ${BOOTKUBE_REPO}:${BOOTKUBE_VERSION} --exec \
        /bootkube -- start --asset-dir=/core/assets
}

[ "$#" == 1 ] || usage

[ -d "${CLUSTER_DIR}" ] && {
    echo "Error: CLUSTER_DIR=${CLUSTER_DIR} already exists"
    exit 1
}

# This script can execute on a remote host by copying itself + kubelet service unit to remote host.
# After assets are available on the remote host, the script will execute itself in "local" mode.
if [ "${REMOTE_HOST}" != "local" ]; then
    # Set up the kubelet.service on remote host
    scp -i ${IDENT} -P ${REMOTE_PORT} ${SSH_OPTS} kubelet.master core@${REMOTE_HOST}:/home/core/kubelet.master
    ssh -i ${IDENT} -p ${REMOTE_PORT} ${SSH_OPTS} core@${REMOTE_HOST} "sudo mv /home/core/kubelet.master /etc/systemd/system/kubelet.service"

    # Copy self to remote host so script can be executed in "local" mode
    scp -i ${IDENT} -P ${REMOTE_PORT} ${SSH_OPTS} ${BASH_SOURCE[0]} core@${REMOTE_HOST}:/home/core/init-master.sh
    ssh -i ${IDENT} -p ${REMOTE_PORT} ${SSH_OPTS} core@${REMOTE_HOST} "sudo BOOTKUBE_REPO=${BOOTKUBE_REPO} BOOTKUBE_VERSION=${BOOTKUBE_VERSION} /home/core/init-master.sh local"

    # Copy assets from remote host to a local directory. These can be used to launch additional nodes & contain TLS assets
    mkdir ${CLUSTER_DIR}
    scp -q -i ${IDENT} -P ${REMOTE_PORT} ${SSH_OPTS} -r core@${REMOTE_HOST}:/home/core/assets/* ${CLUSTER_DIR}

    # Cleanup
    ssh -i ${IDENT} -p ${REMOTE_PORT} ${SSH_OPTS} core@${REMOTE_HOST} "rm -rf /home/core/assets && rm -rf /home/core/init-master.sh"

    echo "Cluster assets copied to ${CLUSTER_DIR}"
    echo
    echo "Bootstrap complete. Access your kubernetes cluster using:"
    echo "kubectl --kubeconfig=${CLUSTER_DIR}/auth/kubeconfig get nodes"
    echo
    echo "Additional nodes can be added to the cluster using:"
    echo "./init-worker.sh <node-ip> ${CLUSTER_DIR}/auth/kubeconfig"
    echo

# Execute this script locally on the machine, assumes a kubelet.service file has already been placed on host.
elif [ "$1" == "local" ]; then
    init_master_node
fi
