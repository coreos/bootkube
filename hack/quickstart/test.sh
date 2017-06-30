
#!/usr/bin/env bash
set -euo pipefail

export SSH_OPTS=${SSH_OPTS:-}" -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no"

echo $SSH_OPTS
