#!/bin/bash
set -euo pipefail

REMOTE_HOST=$1
REMOTE_PORT=${REMOTE_PORT:-22}
CLUSTER_DIR=${CLUSTER_DIR:-cluster}
IDENT=${IDENT:-${HOME}/.ssh/id_rsa}

COREOS_ENV_FILE=${COREOS_ENV_FILE:-/var/coreos/metadata}
ETCD_DROPIN_FILE=10-coreos-k8s-etcd.conf
SEDSTRING=""

if [ -f $COREOS_ENV_FILE ]; then
  source $COREOS_ENV_FILE
fi 

if [ -f /etc/os-release ]; then
  source /etc/os-release
  OS_NAME=$ID
  OS_VER_MAJOR=${VERSION_ID%.*}
else
  echo "Your distribution does not have an /etc/os-release file, which means it's unsupported for Kubernetes self-hosted install."
  exit 1
fi

BOOTKUBE_REPO=quay.io/coreos/bootkube
BOOTKUBE_VERSION=v0.1.4

function usage() {
    echo "USAGE:"
    echo "$0: <remote-host>"
    exit 1
}

function configure_etcd() {

    if [ -f /lib/systemd/system/etcd2.service ]; then
      ETCD_DROPIN_DIR=/etc/systemd/system/etcd2.service.d
      ETCD_UNIT_FILE="etcd2.service"
    elif [ -f /lib/systemd/system/etcd.service ]; then
      ETCD_DROPIN_DIR=/etc/systemd/system/etcd.service.d
      ETCD_UNIT_FILE="etcd.service"
      SEDSTRING="s#etcd2.service#etcd.service#;"
    else
      echo "No etcd service installed, terminating."
    fi

    [ -f "${ETCD_DROPIN_DIR}/${ETCD_DROPIN_FILE}" ] || {
        mkdir -p $ETCD_DROPIN_DIR
        cat << EOF > ${ETCD_DROPIN_DIR}/${ETCD_DROPIN_FILE}
[Service]
EnvironmentFile=
Environment="ETCD_NAME=controller"
Environment="ETCD_INITIAL_CLUSTER=controller=http://${COREOS_PRIVATE_IPV4}:2380"
Environment="ETCD_INITIAL_ADVERTISE_PEER_URLS=http://${COREOS_PRIVATE_IPV4}:2380"
Environment="ETCD_ADVERTISE_CLIENT_URLS=http://${COREOS_PRIVATE_IPV4}:2379"
Environment="ETCD_LISTEN_CLIENT_URLS=http://0.0.0.0:2379"
Environment="ETCD_LISTEN_PEER_URLS=http://0.0.0.0:2380"
EOF
    }
}

function configure_flannel() {
    # Configure Flannel options
    [ -f "/etc/flannel/options.env" ] || {
        mkdir -p /etc/flannel
        echo "FLANNELD_IFACE=${COREOS_PRIVATE_IPV4}" >> /etc/flannel/options.env
        echo "FLANNELD_ETCD_ENDPOINTS=http://127.0.0.1:2379" >> /etc/flannel/options.env
    }

    # Make sure options are re-used on reboot
    local TEMPLATE=/etc/systemd/system/flanneld.service.d/10-symlink.conf.conf
    [ -f $TEMPLATE ] || {
        mkdir -p $(dirname $TEMPLATE)
        echo "[Service]" >> $TEMPLATE
        echo "ExecStartPre=${LNPATH} -sf /etc/flannel/options.env /run/flannel/options.env" >> $TEMPLATE
    }
}

# wait until etcd is available, then configure the flannel pod-network settings.
function configure_network() {
    while true; do
        echo "Waiting for etcd..."
        /usr/bin/etcdctl cluster-health && break
        sleep 1
    done
    /usr/bin/etcdctl set /coreos.com/network/config '{ "Network": "10.2.0.0/16", "Backend":{"Type":"vxlan"}}'
}

# Initialize a Master node
function init_master_node() {
    if [ "$OS_NAME" = "coreos" ]; then
      systemctl stop update-engine; systemctl mask update-engine
    fi

    # Start etcd and configure network settings
    configure_etcd
    configure_flannel
    systemctl daemon-reload
    systemctl enable $ETCD_UNIT_FILE; systemctl start $ETCD_UNIT_FILE
    configure_network

    # Start flannel
    systemctl enable flanneld; systemctl start flanneld

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

    # Set up the kubelet service

    SEDSTRING=${SEDSTRING}"s#{{COREOS_ENV_FILE}}#$COREOS_ENV_FILE#"

    sed -e $SEDSTRING /home/core/kubelet.master > /etc/systemd/system/kubelet.service

    # Start the kubelet
    systemctl enable kubelet; systemctl start kubelet

    # Start bootkube to launch a self-hosted cluster
    /usr/bin/rkt run \
        --volume home,kind=host,source=/home/core \
        --mount volume=home,target=/core \
        --net=host ${BOOTKUBE_REPO}:${BOOTKUBE_VERSION} --exec \
        /bootkube -- start --asset-dir=/core/assets
}

[ "$#" -eq 1 ] || usage

[ -d "${CLUSTER_DIR}" ] && {
    echo "Error: CLUSTER_DIR=${CLUSTER_DIR} already exists"
    exit 1
}

# This script can execute on a remote host by copying itself + kubelet service unit to remote host.
# After assets are available on the remote host, the script will execute itself in "local" mode.
if [ "${REMOTE_HOST}" != "local" ]; then
    # Set up the kubelet.service on remote host
    scp -i ${IDENT} -P ${REMOTE_PORT} kubelet.master core@${REMOTE_HOST}:/home/core/kubelet.master
    # ssh -t -i ${IDENT} -p ${REMOTE_PORT} core@${REMOTE_HOST} "sudo mv /home/core/kubelet.master /etc/systemd/system/kubelet.service"

    # Copy self to remote host so script can be executed in "local" mode
    scp -i ${IDENT} -P ${REMOTE_PORT} ${BASH_SOURCE[0]} core@${REMOTE_HOST}:/home/core/init-master.sh
    ssh -t -i ${IDENT} -p ${REMOTE_PORT} core@${REMOTE_HOST} "sudo /home/core/init-master.sh local"

    # Copy assets from remote host to a local directory. These can be used to launch additional nodes & contain TLS assets
    mkdir ${CLUSTER_DIR}
    scp -q -i ${IDENT} -P ${REMOTE_PORT} -r core@${REMOTE_HOST}:/home/core/assets/* ${CLUSTER_DIR}

    # Cleanup
    ssh -i ${IDENT} -p ${REMOTE_PORT} core@${REMOTE_HOST} "rm -rf /home/core/assets && rm -rf /home/core/init-master.sh"

    echo "Cluster assets copied to ${CLUSTER_DIR}"
    echo
    echo "Bootstrap complete. Access your kubernetes cluster using:"
    echo "kubectl --kubeconfig=${CLUSTER_DIR}/auth/kubeconfig get nodes"
    echo
    echo "Additional nodes can be added to the cluster using:"
    echo "./init-worker.sh <node-ip> ${CLUSTER_DIR}/auth/kubeconfig"
    echo

# Execute this script locally on the machine, assumes a kubelet.service file has already been placed on host.
elif [ "$1" = "local" ]; then
    init_master_node
fi
