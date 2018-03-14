#!/usr/bin/env bash
#
# Ensure that all vendored packages are pinned to a specific version. This means
# we never pull HEAD of a dependency, but as a trade-off means we have to manually
# specify the version of transitive dependencies.

set -e

TOML=$( grep -P '( |^)name = ".*"' Gopkg.toml | tr -d ' ' | sort )
LOCK=$( grep -P '( |^)name = ".*"' Gopkg.lock | tr -d ' ' | sort )

# '|| true' because diff exits 1 when there's a difference.
DIFF=$( diff <(echo "$TOML") <(echo "$LOCK") || true )
if [ "$DIFF" != "" ]; then
    >&2 echo "Difference between Gopkg.lock file and Gopkg.toml detected. Ensure all dependencies are specified in Gopkg.toml:"
    >&2 echo ""
    >&2 echo "$DIFF"
    exit 1
fi
