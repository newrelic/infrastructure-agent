#!/bin/bash

set -e

# This script fetches the latest and the one-before-latest stable versions of the
# New Relic Fluent Bit output plugin and prints them for GHA consumption.
#
# It mirrors previous_version.sh (used for the infra-agent), but the versions come
# from the newrelic/newrelic-fluent-bit-output releases instead of local git tags.
#
# Pre-releases and releases whose name contains "bad" are skipped, so the A2Q
# canaries always exercise two shippable versions.

REPO="newrelic/newrelic-fluent-bit-output"

# Newest-first list of stable semver versions (leading "v" stripped).
VERSIONS=$(
  gh release list --repo "$REPO" --exclude-pre-releases --limit 50 \
    --json tagName,isPrerelease,name \
    --jq '.[] | select(.isPrerelease==false) | select((.name | ascii_downcase | contains("bad")) | not) | .tagName' \
    | sed 's/^v//' \
    | grep -E '^[0-9]+\.[0-9]+\.[0-9]+$'
)

if [ "$(echo "$VERSIONS" | wc -l)" -lt 2 ]; then
  echo "Expected at least 2 stable releases in $REPO, found: $VERSIONS" >&2
  exit 1
fi

FB_LATEST_VERSION=$(echo "$VERSIONS" | sed -n '1p')
FB_PREVIOUS_VERSION=$(echo "$VERSIONS" | sed -n '2p')

# Set the variables for later use in the GHA pipeline
{
    echo "FB_LATEST_VERSION=${FB_LATEST_VERSION}"
    echo "FB_PREVIOUS_VERSION=${FB_PREVIOUS_VERSION}"
} >> "$GITHUB_ENV"
