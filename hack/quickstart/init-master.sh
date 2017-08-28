#!/usr/bin/env bash
set -euo pipefail

REMOTE_HOST=$1
REMOTE_PORT=${REMOTE_PORT:-22}
REMOTE_USER=${REMOTE_USER:-core}
CLUSTER_DIR=${CLUSTER_DIR:-cluster}
IDENT=${IDENT:-${HOME}/.ssh/id_rsa}
SSH_OPTS=${SSH_OPTS:-}
MULTI_MASTER=${MULTI_MASTER:-false}
SELF_HOST_ETCD=${SELF_HOST_ETCD:-false}
CALICO_NETWORK_POLICY=${CALICO_NETWORK_POLICY:-false}
CLOUD_PROVIDER=${CLOUD_PROVIDER:-}

function usage() {
    echo "USAGE:"
    echo "$0: <remote-host>"
    exit 1
}

function configure_etcd() {
    [ -f "/etc/systemd/system/etcd-member.service.d/10-etcd-member.conf" ] || {
        mkdir -p /etc/etcd/tls
        cp /home/${REMOTE_USER}/assets/tls/etcd-* /etc/etcd/tls
        mkdir -p /etc/etcd/tls/etcd
        cp /home/${REMOTE_USER}/assets/tls/etcd/* /etc/etcd/tls/etcd
        chown -R etcd:etcd /etc/etcd
        chmod -R u=rX,g=,o= /etc/etcd
        mkdir -p /etc/systemd/system/etcd-member.service.d
        cat << EOF > /etc/systemd/system/etcd-member.service.d/10-etcd-member.conf
[Service]
Environment="ETCD_IMAGE_TAG=v3.1.8"
Environment="ETCD_NAME=controller"
Environment="ETCD_INITIAL_CLUSTER=controller=https://${COREOS_PRIVATE_IPV4}:2380"
Environment="ETCD_INITIAL_ADVERTISE_PEER_URLS=https://${COREOS_PRIVATE_IPV4}:2380"
Environment="ETCD_ADVERTISE_CLIENT_URLS=https://${COREOS_PRIVATE_IPV4}:2379"
Environment="ETCD_LISTEN_CLIENT_URLS=https://0.0.0.0:2379"
Environment="ETCD_LISTEN_PEER_URLS=https://0.0.0.0:2380"
Environment="ETCD_SSL_DIR=/etc/etcd/tls"
Environment="ETCD_TRUSTED_CA_FILE=/etc/ssl/certs/etcd/server-ca.crt"
Environment="ETCD_CERT_FILE=/etc/ssl/certs/etcd/server.crt"
Environment="ETCD_KEY_FILE=/etc/ssl/certs/etcd/server.key"
Environment="ETCD_CLIENT_CERT_AUTH=true"
Environment="ETCD_PEER_TRUSTED_CA_FILE=/etc/ssl/certs/etcd/peer-ca.crt"
Environment="ETCD_PEER_CERT_FILE=/etc/ssl/certs/etcd/peer.crt"
Environment="ETCD_PEER_KEY_FILE=/etc/ssl/certs/etcd/peer.key"
EOF
    }
}

# Initialize a Master node
function init_master_node() {
    systemctl daemon-reload
    systemctl stop locksmithd; systemctl mask locksmithd

    if [ "$SELF_HOST_ETCD" = true ] ; then
        echo "WARNING: THIS IS NOT YET FULLY WORKING - merely here to make ongoing testing easier"
        etcd_render_flags="--experimental-self-hosted-etcd"
    else
        etcd_render_flags="--etcd-servers=https://${COREOS_PRIVATE_IPV4}:2379"
    fi

    if [ "$CALICO_NETWORK_POLICY" = true ]; then
        echo "WARNING: THIS IS EXPERIMENTAL SUPPORT FOR NETWORK POLICY"
        cnp_render_flags="--experimental-calico-network-policy"
    else
        cnp_render_flags=""
    fi

    # Render cluster assets
    /home/${REMOTE_USER}/bootkube render --asset-dir=/home/${REMOTE_USER}/assets ${etcd_render_flags} ${cnp_render_flags} \
      --api-servers=https://${COREOS_PUBLIC_IPV4}:443,https://${COREOS_PRIVATE_IPV4}:443,https://127.0.0.1:6443

    # Move the local kubeconfig into expected location
    chown -R ${REMOTE_USER}:${REMOTE_USER} /home/${REMOTE_USER}/assets
    mkdir -p /etc/kubernetes
    cp /home/${REMOTE_USER}/assets/auth/kubeconfig /etc/kubernetes/
    cp /home/${REMOTE_USER}/assets/tls/ca.crt /etc/kubernetes/ca.crt

    if [ "$MULTI_MASTER" = true ]; then
        sed -r -e "s/(server: ).*/\1https:\/\/127.0.0.1:6443/" -i /etc/kubernetes/kubeconfig
        mkdir -p /etc/nginx
        cp nginx.conf /etc/nginx
        echo "server 127.0.0.1:443 backup;" > /etc/nginx/upstream.conf
        mkdir -p /etc/kubernetes/manifests
        cp nginx-proxy.yaml /etc/kubernetes/manifests
    fi

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
    /home/${REMOTE_USER}/bootkube start --asset-dir=/home/${REMOTE_USER}/assets
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
    scp -i ${IDENT} -P ${REMOTE_PORT} ${SSH_OPTS} kubelet.master ${REMOTE_USER}@${REMOTE_HOST}:/home/${REMOTE_USER}/kubelet.master
    ssh -i ${IDENT} -p ${REMOTE_PORT} ${SSH_OPTS} ${REMOTE_USER}@${REMOTE_HOST} "sudo mv /home/${REMOTE_USER}/kubelet.master /etc/systemd/system/kubelet.service"

    # Copy bootkube binary to remote host.
    if [ -e "../../_output/bin/linux/bootkube" ]; then
        scp -i ${IDENT} -P ${REMOTE_PORT} -C ${SSH_OPTS} ../../_output/bin/linux/bootkube ${REMOTE_USER}@${REMOTE_HOST}:/home/${REMOTE_USER}/bootkube
    else
        echo "Error: Local static bootkube binary not found."
        echo "Falling back to bootkube binary in path."
        if which bootkube &>/dev/null; then
            scp -i ${IDENT} -P ${REMOTE_PORT} -C ${SSH_OPTS} "$(which bootkube)" ${REMOTE_USER}@${REMOTE_HOST}:/home/${REMOTE_USER}/bootkube
        else
            echo "Error: bootkube not found in PATH."
            exit 1
        fi
    fi

    if [ "$MULTI_MASTER" = true ]; then
        scp -i ${IDENT} -P ${REMOTE_PORT} ${SSH_OPTS} nginx.conf ${REMOTE_USER}@${REMOTE_HOST}:/home/${REMOTE_USER}/nginx.conf
        scp -i ${IDENT} -P ${REMOTE_PORT} ${SSH_OPTS} nginx-proxy.yaml ${REMOTE_USER}@${REMOTE_HOST}:/home/${REMOTE_USER}/nginx-proxy.yaml
    fi

    # Copy self to remote host so script can be executed in "local" mode
    scp -i ${IDENT} -P ${REMOTE_PORT} ${SSH_OPTS} ${BASH_SOURCE[0]} ${REMOTE_USER}@${REMOTE_HOST}:/home/${REMOTE_USER}/init-master.sh
    ssh -i ${IDENT} -p ${REMOTE_PORT} ${SSH_OPTS} ${REMOTE_USER}@${REMOTE_HOST} "sudo REMOTE_USER=${REMOTE_USER} CLOUD_PROVIDER=${CLOUD_PROVIDER} MULTI_MASTER=${MULTI_MASTER} SELF_HOST_ETCD=${SELF_HOST_ETCD} CALICO_NETWORK_POLICY=${CALICO_NETWORK_POLICY} /home/${REMOTE_USER}/init-master.sh local"

    # Copy assets from remote host to a local directory. These can be used to launch additional nodes & contain TLS assets
    mkdir ${CLUSTER_DIR}
    scp -q -i ${IDENT} -P ${REMOTE_PORT} ${SSH_OPTS} -r ${REMOTE_USER}@${REMOTE_HOST}:/home/${REMOTE_USER}/assets/* ${CLUSTER_DIR}

    # Cleanup
    ssh -i ${IDENT} -p ${REMOTE_PORT} ${SSH_OPTS} ${REMOTE_USER}@${REMOTE_HOST} "rm -rf /home/${REMOTE_USER}/{assets,init-master.sh,bootkube,nginx.conf,nginx-proxy.yaml}"

    echo "Cluster assets copied to ${CLUSTER_DIR}"
    echo
    echo "Bootstrap complete. Access your kubernetes cluster using:"
    echo "kubectl --kubeconfig=${CLUSTER_DIR}/auth/kubeconfig get nodes"
    echo
    echo "Additional nodes can be added to the cluster using:"
    echo "./init-node.sh <node-ip> ${CLUSTER_DIR}/auth/kubeconfig"
    echo

# Execute this script locally on the machine, assumes a kubelet.service file has already been placed on host.
elif [ "$1" == "local" ]; then
    init_master_node
fi
