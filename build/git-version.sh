#!/bin/sh

DESCRIPTION=$(git describe --abbrev=100 --dirty) &&
echo "${DESCRIPTION##*-g}"
