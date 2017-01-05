#!/bin/bash
set -euo pipefail

REMOTE_HOST=$1
REMOTE_PORT=${REMOTE_PORT:-22}
CLUSTER_DIR=${CLUSTER_DIR:-cluster}
IDENT=${IDENT:-${HOME}/.ssh/id_rsa}
SSH_OPTS=${SSH_OPTS:-}

BOOTKUBE_REPO=${BOOTKUBE_REPO:-quay.io/coreos/bootkube}
BOOTKUBE_VERSION=${BOOTKUBE_VERSION:-v0.3.5}

function usage() {
    echo "USAGE:"
    echo "$0: <remote-host>"
    exit 1
}

function configure_etcd() {
    [ -f "/etc/systemd/system/etcd-member.service.d/10-etcd-member.conf" ] || {
        mkdir -p /etc/systemd/system/etcd-member.service.d
        cat << EOF > /etc/systemd/system/etcd-member.service.d/10-etcd-member.conf
[Service]
Environment="ETCD_IMAGE_TAG=v3.1.0"
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

    # Start etcd.
    configure_etcd
    systemctl enable etcd-member; sudo systemctl start etcd-member

    # Render cluster assets
    /usr/bin/rkt run \
        --volume home,kind=host,source=/home/core \
        --mount volume=home,target=/core \
        --trust-keys-from-https --net=host ${BOOTKUBE_REPO}:${BOOTKUBE_VERSION} --exec \
        /bootkube -- render --asset-dir=/core/assets --api-servers=https://${COREOS_PRIVATE_IPV4}:443,https://${COREOS_PUBLIC_IPV4}:443

    # Move the local bootstrap-kubeconfig into expected location
    chown -R core:core /home/core/assets
    mkdir -p /etc/kubernetes
    cp /home/core/assets/auth/bootstrap-kubeconfig /etc/kubernetes/

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

# This script can execute on a remote host by copying itself and the kubelet service unit to remote host.
# After assets are available on the remote host, the script will execute itself in "local" mode.
if [ "${REMOTE_HOST}" != "local" ]; then
    # Set up the kubelet.service on remote host
    scp -i ${IDENT} -P ${REMOTE_PORT} ${SSH_OPTS} kubelet.master core@${REMOTE_HOST}:/home/core/kubelet.master
    ssh -i ${IDENT} -p ${REMOTE_PORT} ${SSH_OPTS} core@${REMOTE_HOST} "sudo mv /home/core/kubelet.master /etc/systemd/system/kubelet.service"

    # Copy self to remote host and execute script in "local" mode
    scp -i ${IDENT} -P ${REMOTE_PORT} ${SSH_OPTS} ${BASH_SOURCE[0]} core@${REMOTE_HOST}:/home/core/init-master.sh
    ssh -i ${IDENT} -p ${REMOTE_PORT} ${SSH_OPTS} core@${REMOTE_HOST} "sudo BOOTKUBE_REPO=${BOOTKUBE_REPO} BOOTKUBE_VERSION=${BOOTKUBE_VERSION} /home/core/init-master.sh local"

    # Copy assets from remote host to a local directory. These can be used to launch additional nodes & contain TLS assets
    mkdir ${CLUSTER_DIR}
    scp -q -i ${IDENT} -P ${REMOTE_PORT} ${SSH_OPTS} -r core@${REMOTE_HOST}:/home/core/assets/* ${CLUSTER_DIR}
    sed -i.private "s/server: .*/server: https:\/\/${REMOTE_HOST}:443\//" ${CLUSTER_DIR}/auth/admin-kubeconfig

    # Cleanup
    ssh -i ${IDENT} -p ${REMOTE_PORT} ${SSH_OPTS} core@${REMOTE_HOST} "sudo rm -rf /home/core/assets /home/core/init-master.sh"

    echo "Cluster assets copied to ${CLUSTER_DIR}"
    echo
    echo "Bootstrap complete. Access your kubernetes cluster using:"
    echo "kubectl --kubeconfig=${CLUSTER_DIR}/auth/admin-kubeconfig get nodes"
    echo
    echo "Additional nodes can be added to the cluster using:"
    echo "./init-worker.sh <node-ip>"
    echo

# Execute this script locally on the machine, assumes a kubelet.service file has already been placed on host.
elif [ "$1" == "local" ]; then
    init_master_node
fi
