#!/usr/bin/env bash
set -x
set -euo pipefail

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

#export CLUSTER_NAME="${CLUSTER_NAME:-"bootkube$(printf '0x%x' $(date +%s))"}"
export CLUSTER_NAME="${CLUSTER_NAME:-"default"}"
export CLUSTER_DIR="${CLUSTER_DIR:-"${DIR}/../_clusters/${CLUSTER_NAME}"}"
mkdir -p "${CLUSTER_DIR}"

export NUM_WORKERS=${NUM_WORKERS:-1}
export REGION="${REGION:-"us-west-2"}"

export TERRAFORM_VERSION="0.11.3"

# variables from ../quickstart/init-*.sh
#REMOTE_HOST=$1
#REMOTE_PORT=${REMOTE_PORT:-22}
#REMOTE_USER=${REMOTE_USER:-core}
#CLUSTER_DIR=${CLUSTER_DIR:-cluster}
#IDENT=${IDENT:-${HOME}/.ssh/id_rsa}
#SSH_OPTS=${SSH_OPTS:-}
#CLOUD_PROVIDER=${CLOUD_PROVIDER:-}
#NETWORK_PROVIDER=${NETWORK_PROVIDER:-flannel}

# generate ssh-key
ssh-keygen -t rsa -f "${CLUSTER_DIR}/id_rsa" -q -N ""
if [ -z "${SSH_AUTH_SOCK:-}" ] ; then
  ssh-agent -s > "${CLUSTER_DIR}/sshagent.env"
  source "${CLUSTER_DIR}/sshagent.env"
  ssh-add "${CLUSTER_DIR}/id_rsa"
fi

if [[ -f "../../_output/bin/linux/bootkube" ]]; then
    echo "Build already exists. Skipping (re)build."
else
    ../../build/build-release.sh
fi

# install terraform
if [[ ! -f "${DIR}/bin/terraform" ]]; then
    (
        mkdir -p "${DIR}/bin/"
        cd "${DIR}/bin/";
        curl -L -O "https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_linux_amd64.zip"
        unzip "terraform_${TERRAFORM_VERSION}_linux_amd64.zip"
        rm "terraform_${TERRAFORM_VERSION}_linux_amd64.zip"
    )
fi
export TERRAFORM="${DIR}/bin/terraform"

cd ../terraform-quickstart

export TF_VAR_access_key_id="${ACCESS_KEY_ID}"
export TF_VAR_access_key="${ACCESS_KEY_SECRET}"
#export TF_VAR_instance_tags="${CLUSTER_NAME}" # unused??
export TF_VAR_resource_owner="${CLUSTER_NAME}"
export TF_VAR_ssh_public_key="$(cat ${CLUSTER_DIR}/id_rsa.pub)"
export TF_VAR_ssh_key="jenkins-core-services"
export TF_VAR_additional_masters=0
export TF_VAR_num_workers=${NUM_WORKERS}
export TF_VAR_region="${REGION}"

export TERRAFORM_STATE_FILE="${CLUSTER_DIR}/terraform.tfstate"

# bring up compute
terraform init
"${TERRAFORM}" apply --auto-approve --state "${TERRAFORM_STATE_FILE}"

# sleep so ssh works with start-cluster
sleep 30

#avoid some IPs being blank bootkube/issues/552
"${TERRAFORM}" refresh --state "${TERRAFORM_STATE_FILE}"

#launch bootkube via quickstart scripts
./start-cluster.sh

if [[ "${RUN_E2E:-}" == "y" ]]; then
    #run tests -- mount /tmp for access to ssh agent 
    BOOTKUBE_ROOT=$(git rev-parse --show-toplevel)
    sudo rkt run \
        --volume bk,kind=host,source=${BOOTKUBE_ROOT} \
        --mount volume=bk,target=/go/src/github.com/kubernetes-incubator/bootkube \
        --volume tmp,kind=host,source=/tmp \
        --mount volume=tmp,target=/tmp \
        --set-env SSH_AUTH_SOCK=$SSH_AUTH_SOCK \
        --insecure-options=image docker://golang:1.8.3 --exec /bin/bash -- -c \
        "cd /go/src/github.com/kubernetes-incubator/bootkube/e2e && \
        go test -v -timeout 45m \
        --kubeconfig=../hack/quickstart/cluster/auth/kubeconfig \
        --expectedmasters=1 \
        ./e2e/"
fi
