#!/bin/bash
set -e
#
#
# Upload msi artifacts to GH Release assets
#
#
ARCH=$1
TAG=$2

hub release edit -a "build/package/windows/newrelic-infra-${ARCH}-installer/newrelic-infra/bin/Release/newrelic-infra-${ARCH}.msi" -m ${TAG} ${TAG}