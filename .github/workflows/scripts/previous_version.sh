#!/bin/bash

set -e

# this script accetps a tag as input and it will search and output for the previous one
# for GHA
# if no tag is passed as parameter, the latest one will be used

# fetch the history (including tags) from within a shallow clone like CI-GHA
# supress error when the repository is a complete one.
git fetch --prune --unshallow 2> /dev/null || true

TAG=$1
if [ -z $TAG ];then
  TAG=$( git tag | grep -E "^[0-9]+\.[0-9]+\.[0-9]$" | sort | tail -n 1 )
fi

# print previous tag
PREVIOUS_TAG=$( git tag | grep -E "^[0-9]+\.[0-9]+\.[0-9]$" | sort | grep -B 1 $TAG | head -n 1 )

while true; do
    # Get release name from current PREVIOUS_TAG
    PREV_RELEASE_NAME=$(gh release view $PREVIOUS_TAG --json name --jq .name)

    if [[ "$(echo "$PREV_RELEASE_NAME" | tr '[:upper:]' '[:lower:]')" == *"bad"* ]]; then
        # Update PREVIOUS_TAG to be the tag before the current one
        PREVIOUS_TAG=$(git describe --tags --abbrev=0 $PREVIOUS_TAG^)
    else
        break
    fi
done

echo "PREVIOUS_TAG=$PREVIOUS_TAG"
