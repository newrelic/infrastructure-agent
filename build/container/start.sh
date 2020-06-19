#!/usr/bin/env bash

set -e


# Args:
#   - $1: newrelic-infra agent container name
#   - $2: newrelic-infra agent docker image name
#   - $3: Agent display name

CONTAINER_NAME=$1
IMAGE_NAME=$2
DISPLAY_NAME=$3

# Ensure CONTAINER_NAME is set & non-empty
if [ -z "$CONTAINER_NAME" ]; then
	echo "CONTAINER_NAME is not set or empty"
	exit 1
fi

# Ensure IMAGE_NAME is set & non-empty
if [ -z "$IMAGE_NAME" ]; then
	echo "IMAGE_NAME is not set or empty"
	exit 1
fi

# Ensure DISPLAY_NAME is set & non-empty
if [ -z "$DISPLAY_NAME" ]; then
	echo "DISPLAY_NAME is not set or empty"
	exit 1
fi

docker run \
	--name $CONTAINER_NAME \
	--privileged=true \
	--uts="host" \
	--volume=/proc:/host/proc \
	-e "HOST_PROC=/host/proc" \
	--volume=/sys:/host/sys \
	-e "HOST_SYS=/host/sys" \
	--volume=/etc:/host/etc \
	-e "HOST_ETC=/host/etc" \
	-e "NRIA_DISPLAY_NAME=$DISPLAY_NAME" \
	$IMAGE_NAME
