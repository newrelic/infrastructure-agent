#!/bin/bash

set -e

# this script acceps a tag as input and it will search for the previous one and set both as env vars
# for GHA
# if no tag is passed as parameter, the latest one will be used

# fetch the history (including tags) from within a shallow clone like CI-GHA
# supress error when the repository is a complete one.
git fetch --prune --unshallow 2> /dev/null || true

TAG=$1
if [ -z $TAG ];then
  TAG=$( git tag | grep -E "^[0-9]+\.[0-9]+\.[0-9]$" | sort | tail -n 1 )
fi

PREVIOUS_TAG=1.65.4

# Set the variables for later use in the GHA pipeline
{
    echo "NR_VERSION=${TAG}"
    echo "PREVIOUS_NR_VERSION=${PREVIOUS_TAG}"
} >> "$GITHUB_ENV"
