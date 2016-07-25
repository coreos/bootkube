#!/bin/bash
set -euo pipefail

ssh_key=$(mktemp)
ip=$(corectl q -i api-server-1)
chmod 700 ${ssh_key}
(corectl q api-server-1 -j | jq -r ".[] | .InternalSSHprivate") > ${ssh_key}
echo ../tests/conformance-test.sh api-server-1 22 ${ssh_key}
../tests/conformance-test.sh ${ip} 22 ${ssh_key}
rm -rf ${ssh_key}