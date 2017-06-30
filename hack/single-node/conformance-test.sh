#!/usr/bin/env bash
set -euo pipefail

ssh_key="$(vagrant ssh-config | awk '/IdentityFile/ {print $2}' | tr -d '"')"
ssh_port="$(vagrant ssh-config | awk '/Port [0-9]+/ {print $2}')"

../tests/conformance-test.sh "127.0.0.1" "${ssh_port}" "${ssh_key}"
