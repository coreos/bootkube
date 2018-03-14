#!/usr/bin/env bash
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
set -x
set -euo pipefail

export IDENT="${IDENT:-"${HOME}/.ssh/id_rsa"}"

cd ${DIR}/../../terraform-quickstart/

# TODO: unclear why ssh-agent isn't still running from tqs-up.sh...
if [ -z "${SSH_AUTH_SOCK:-}" ] ; then
  ssh-agent -s > "/tmp/bootkube-tqs-sshagent.env"
  source "/tmp/bootkube-tqs-sshagent.env"
  ssh-add "${IDENT}"
fi

./genlogs.sh
./copylogs.sh
