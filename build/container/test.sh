#!/usr/bin/env bash

set -e
set -x

#  Environment variables:
#
#    - Required:
#      - PROJECT_ROOT: absolute path to the project root dir
#
#    - Optional:
#     - IMAGE_NAME: newrelic-infra agent Docker image, defaults to "newrelic-infra"
#     - CONTAINER_NAME: newrelic-infra agent container name, defaults to "newrelic-infra"

# Ensure project root is set & non-empty
if [ -z "$PROJECT_ROOT" ]; then
	echo "PROJECT_ROOT is not set or empty"
	exit 1
fi

: ${IMAGE_NAME:='newrelic-infra'}
: ${CONTAINER_NAME:='newrelic-infra'}

# Change working dir to project root
cd $PROJECT_ROOT

# Create relative workspace dir for temp build artifacts
WORKSPACE="./workspace"
mkdir -p $WORKSPACE

TEST_DURATION_SEC=15

AGENT_BIN="${WORKSPACE}/newrelic-infra"
AGENT_CFG="${WORKSPACE}/newrelic-infra.yml"
AGENT_DISPLAY_NAME_VM="host_agent"
AGENT_DISPLAY_NAME_CONTAINER="container_agent"
AGENT_LOG_VM="${WORKSPACE}/newrelic-infra-host.log"
AGENT_LOG_CONTAINER="${WORKSPACE}/newrelic-infra-container.log"

echo "Running test for $TEST_DURATION_SEC seconds..."

# Start a detached host agent
sudo NRIA_DISPLAY_NAME=$AGENT_DISPLAY_NAME_VM \
	$AGENT_BIN \
	--config $AGENT_CFG \
	&> $AGENT_LOG_VM \
	&

# Start a detached container agent
./start.sh \
	$CONTAINER_NAME \
	$IMAGE_NAME \
	$AGENT_DISPLAY_NAME_CONTAINER \
	> $AGENT_LOG_CONTAINER \
	&

sleep $TEST_DURATION_SEC

echo "Stopping test..."

./stop.sh $CONTAINER_NAME
