#!/usr/bin/env bash

set -e
set -x

#  Environment variables:
#
#    - Required:
#      - PROJECT_ROOT: absolute path to the project root dir
#      - BASE_IMAGE_NAME: base image name for infra agent image
#      - IMAGE_TAG: name to tag the resulting image
#

# Ensure PROJECT_ROOT is set & non-empty
if [ -z "$PROJECT_ROOT" ]; then
	echo "PROJECT_ROOT is not set or empty"
	exit 1
fi

# Ensure BASE_IMAGE_NAME is set & non-empty
if [ -z "$BASE_IMAGE_NAME" ]; then
	echo "BASE_IMAGE_NAME is not set or empty"
	exit 1
fi

# Ensure IMAGE_TAG is set & non-empty
if [ -z "$IMAGE_TAG" ]; then
	echo "IMAGE_TAG is not set or empty"
	exit 1
fi

# Change working dir to project root
cd $PROJECT_ROOT

# Create relative workspace dir for temp build artifacts
WORKSPACE="workspace"
mkdir -p $WORKSPACE

AGENT_CFG="${WORKSPACE}/newrelic-infra.yml"
if [ ! -f $AGENT_CFG ]; then
	echo "AGENT_CFG is not set or empty"
	exit 1
fi


# Build the Docker image passing relative paths to the workspace build files.
# They must be relative because Docker build doesn't allow absolute paths.
docker build \
	--build-arg AGENT_BIN_FILE=$WORKSPACE_AGENT_BIN \
	--build-arg AGENT_CFG_FILE=$WORKSPACE_AGENT_CFG \
	--tag=$IMAGE_NAME \
	.
