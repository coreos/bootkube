#!/usr/bin/env bash
set -euo pipefail

export BOOTSTRAP_IP=`terraform output bootstrap_node_ip`
export WORKER_IPS=`terraform output -json worker_ips | jq -r '.value[]'`
export MASTER_IPS=`terraform output -json master_ips | jq -r '.value[]'`
export SELF_HOST_ETCD=`terraform output self_host_etcd`
export CALICO_NETWORK_POLICY=`terraform output calico_network_policy`
export SSH_OPTS=${SSH_OPTS:-}" -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no"
export CLOUD_PROVIDER=${CLOUD_PROVIDER:-aws}

# Normally we want to default to aws here since that is all terraform
# supports and it is required for the e2e tests. However because of an
# upstream bug, conformance tests won't pass with cloud provider integration
# set to aws. So we need a knob to set the CLOUD_PROVIDER to nothing while
# keeping aws as the default as to not mess up people using the e2e tests. 
if [ "$CLOUD_PROVIDER" == "none" ] ; then
	echo "cloud provider integration disabled"
	CLOUD_PROVIDER=
fi

cd ../quickstart
./init-master.sh $BOOTSTRAP_IP

for IP in $WORKER_IPS
do
  ./init-node.sh $IP cluster/auth/kubeconfig
done

for IP in $MASTER_IPS
do
  TAG_MASTER=true ./init-node.sh $IP cluster/auth/kubeconfig
done
