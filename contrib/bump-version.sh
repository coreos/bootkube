#!/bin/bash
#
# This script will go through each of the tracked files in this repo and update
# the CURRENT_VERSION to the TARGET_VERSION. This is meant as a helper - but
# probably should still double-check the changes are correct

if [ $# -ne 1 ]; then
    echo "USAGE: $0 <target-version>"
    echo "  example: $0 'v1.4.0-alpha.3_coreos.0'"
    exit 1
fi

CURRENT_VERSION=${CURRENT_VERSION:-"v1.4.0-alpha.3_coreos.0"}
TARGET_VERSION=${1}

GIT_ROOT=$(git rev-parse --show-toplevel)

cd $GIT_ROOT
TRACKED=($(git grep -F "${CURRENT_VERSION}"| awk -F : '{print $1}' | sort -u))
for i in "${TRACKED[@]}"; do
    echo Updating $i
    if [ "$(uname -s)" == "Darwin" ]; then
        sed -i "" "s/${CURRENT_VERSION}/${TARGET_VERSION}/g" $i
    else
        sed -i "s/${CURRENT_VERSION}/${TARGET_VERSION}/g" $i
    fi
done

