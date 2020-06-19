#!/usr/bin/env bash

set -e
set -x

# Args:
#   - $1: newrelic-infra agent container name

CONTAINER_NAME=$1

# Ensure CONTAINER_NAME is set & non-empty
if [ -z "$CONTAINER_NAME" ]; then
	echo "CONTAINER_NAME is not set or empty"
	exit 1
fi

docker stop $CONTAINER_NAME
docker rm -v $CONTAINER_NAME

sudo pkill newrelic-infra