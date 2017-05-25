#!/bin/bash
set -euo pipefail

REMOTE_HOST=$1
REMOTE_PORT=${REMOTE_PORT:-22}
CLUSTER_DIR=${CLUSTER_DIR:-cluster}
IDENT=${IDENT:-${HOME}/.ssh/id_rsa}
SSH_OPTS=${SSH_OPTS:-}
SELF_HOST_ETCD=${SELF_HOST_ETCD:-false}
CLOUD_PROVIDER=${CLOUD_PROVIDER:-}

function usage() {
    echo "USAGE:"
    echo "$0: <remote-host>"
    exit 1
}

function configure_etcd() {
    [ -f "/etc/systemd/system/etcd-member.service.d/10-etcd-member.conf" ] || {
        mkdir -p /etc/etcd/tls
        cp /home/core/assets/tls/etcd* /etc/etcd/tls
        chown -R etcd:etcd /etc/etcd
        chmod -R u=rX,g=,o= /etc/etcd
        mkdir -p /etc/systemd/system/etcd-member.service.d
        cat << EOF > /etc/systemd/system/etcd-member.service.d/10-etcd-member.conf
[Service]
Environment="ETCD_IMAGE_TAG=v3.1.6"
Environment="ETCD_NAME=controller"
Environment="ETCD_INITIAL_CLUSTER=controller=https://${COREOS_PRIVATE_IPV4}:2380"
Environment="ETCD_INITIAL_ADVERTISE_PEER_URLS=https://${COREOS_PRIVATE_IPV4}:2380"
Environment="ETCD_ADVERTISE_CLIENT_URLS=https://${COREOS_PRIVATE_IPV4}:2379"
Environment="ETCD_LISTEN_CLIENT_URLS=https://0.0.0.0:2379"
Environment="ETCD_LISTEN_PEER_URLS=https://0.0.0.0:2380"
Environment="ETCD_SSL_DIR=/etc/etcd/tls"
Environment="ETCD_TRUSTED_CA_FILE=/etc/ssl/certs/etcd-ca.crt"
Environment="ETCD_CERT_FILE=/etc/ssl/certs/etcd-client.crt"
Environment="ETCD_KEY_FILE=/etc/ssl/certs/etcd-client.key"
Environment="ETCD_CLIENT_CERT_AUTH=true"
Environment="ETCD_PEER_TRUSTED_CA_FILE=/etc/ssl/certs/etcd-ca.crt"
Environment="ETCD_PEER_CERT_FILE=/etc/ssl/certs/etcd-peer.crt"
Environment="ETCD_PEER_KEY_FILE=/etc/ssl/certs/etcd-peer.key"
EOF
    }
}

# Initialize a Master node
function init_master_node() {
    systemctl daemon-reload
    systemctl stop update-engine; systemctl mask update-engine

    if [ "$SELF_HOST_ETCD" = true ] ; then
        echo "WARNING: THIS IS NOT YET FULLY WORKING - merely here to make ongoing testing easier"
        etcd_render_flags="--experimental-self-hosted-etcd"
    else
        etcd_render_flags="--etcd-servers=https://${COREOS_PRIVATE_IPV4}:2379"
    fi

    # Some systems may not have one or the other variable set
    COREOS_PUBLIC_IPV4=${COREOS_PUBLIC_IPV4:-${COREOS_PRIVATE_IPV4:-127.0.0.1}}
    COREOS_PRIVATE_IPV4=${COREOS_PRIVATE_IPV4:-${COREOS_PUBLIC_IPV4}}

    # Render cluster assets
    /home/core/bootkube render --asset-dir=/home/core/assets ${etcd_render_flags} \
      --api-servers=https://${COREOS_PUBLIC_IPV4}:443,https://${COREOS_PRIVATE_IPV4}:443

    # Move the local kubeconfig into expected location
    chown -R core:core /home/core/assets
    mkdir -p /etc/kubernetes
    cp /home/core/assets/auth/kubeconfig /etc/kubernetes/
    cp /home/core/assets/tls/ca.crt /etc/kubernetes/ca.crt

    # Start etcd.
    if [ "$SELF_HOST_ETCD" = false ] ; then
        configure_etcd
        systemctl enable etcd-member; sudo systemctl start etcd-member
    fi

    # Set cloud provider
    sed -i "s/cloud-provider=/cloud-provider=$CLOUD_PROVIDER/" /etc/systemd/system/kubelet.service

    # Start the kubelet
    systemctl enable kubelet; sudo systemctl start kubelet

    # Start bootkube to launch a self-hosted cluster
    /home/core/bootkube start --asset-dir=/home/core/assets
}

[ "$#" == 1 ] || usage

[ -d "${CLUSTER_DIR}" ] && {
    echo "Error: CLUSTER_DIR=${CLUSTER_DIR} already exists"
    exit 1
}

# This script can execute on a remote host by copying itself + bootkube binary + kubelet service unit to remote host.
# After assets are available on the remote host, the script will execute itself in "local" mode.
if [ "${REMOTE_HOST}" != "local" ]; then
    # Set up the kubelet.service on remote host
    scp -i ${IDENT} -P ${REMOTE_PORT} ${SSH_OPTS} kubelet.master core@${REMOTE_HOST}:/home/core/kubelet.master
    ssh -i ${IDENT} -p ${REMOTE_PORT} ${SSH_OPTS} core@${REMOTE_HOST} "sudo mv /home/core/kubelet.master /etc/systemd/system/kubelet.service"

    # Copy bootkube binary to remote host.
    scp -i ${IDENT} -P ${REMOTE_PORT} ${SSH_OPTS} ../../_output/bin/linux/bootkube core@${REMOTE_HOST}:/home/core/bootkube

    # Copy self to remote host so script can be executed in "local" mode
    scp -i ${IDENT} -P ${REMOTE_PORT} ${SSH_OPTS} ${BASH_SOURCE[0]} core@${REMOTE_HOST}:/home/core/init-master.sh
    ssh -i ${IDENT} -p ${REMOTE_PORT} ${SSH_OPTS} core@${REMOTE_HOST} "sudo CLOUD_PROVIDER=${CLOUD_PROVIDER} SELF_HOST_ETCD=${SELF_HOST_ETCD} /home/core/init-master.sh local"

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
