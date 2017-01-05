#!/bin/bash

# List existing CSRs
listCSR() {
  kubectl --kubeconfig=$KUBECONFIG get csr -o yaml
}

# Get a specific CSR by name
getCSR() {
  kubectl --kubeconfig=$KUBECONFIG get csr $CSR_NAME
}

# Approve a CSR
approveCSR() {
  local object=`curl -sNk --cert <(echo "$CERT") --key <(echo "$KEY") "${API}/${CSR_NAME}"`
  local approved=`echo "${object}" | jq -cr ".status.conditions = [{"type":\"Approved\"}]"`
  echo -ne "${approved}" | curl -k --cert <(echo "$CERT") --key <(echo "$KEY") -X PUT -H "Content-Type: application/json" --data @- "${API}/${CSR_NAME}/approval"
}

# Deny a CSR
denyCSR() {
  local object=`curl -sNk --cert <(echo "$CERT") --key <(echo "$KEY") "${API}/${CSR_NAME}"`
  local denied=`echo "${object}" | jq -cr ".status.conditions = [{"type":\"Denied\"}]"`
  echo -ne "${denied}" | curl -k --cert <(echo "$CERT") --key <(echo "$KEY") -X PUT -H "Content-Type: application/json" --data @- "${API}/${CSR_NAME}/approval"
}

# Show script usage
showUsage() {
  echo "usage: csrctl.sh <kubeconfig-path> [list|get|approve|deny] <csr-name>"
}

KUBECONFIG=$1
CMD=$2
CSR_NAME=$3

if [[ "$#" -ne 2 && "${CMD}" == "list" ]] || [[ "$#" -ne 3 && "${CMD}" != "list" ]]; then
  showUsage
  exit 1
fi

APISERVER=$(grep "server:" ${KUBECONFIG} | cut -f2- -d':' | tr -d " ")
CERT=$(grep "client-certificate-data:" ${KUBECONFIG} | cut -f2- -d':' | tr -d " " | base64 -d)
KEY=$(grep "client-key-data:" ${KUBECONFIG} | cut -f2- -d':' | tr -d " " | base64 -d)
API="${APISERVER}/apis/certificates.k8s.io/v1alpha1/certificatesigningrequests"

if [[ -z "${APISERVER}" ]] || [[ -z "${CERT}" ]] || [[ -z "${KEY}" ]]; then
    echo "Error: Could not parse kubeconfig"
    exit 1
fi

case "$2" in
  list) listCSR;;
  get) getCSR;;
  approve) approveCSR;;
  deny) denyCSR;;
  *) showUsage; exit 1;
esac
