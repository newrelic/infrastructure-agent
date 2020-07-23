.DEFAULT_GOAL := build/base

INFRASTRUCTURE_WORKSPACE       ?= workspace
PROJECT_WORKSPACE              ?= $(CURDIR)/$(INFRASTRUCTURE_WORKSPACE)

TARGET_DIR						= $(PROJECT_WORKSPACE)/target
NRI_INTEGRATIONS_FILE			= ../../build/nri-integrations
NRI_PKG_DIR						?= ${PROJECT_WORKSPACE}/${BASE_OHIS}
AGENT_BUILD_TARGET_DIR			?= ../../target/bin/linux_amd64
ASSETS_DIR						?= ../../

include ../../build/embed_ohis.mk

# AGENT_BUILD_NUMBER comes from the jenkins job
AGENT_BUILD_NUMBER			   ?= 0.0
IMAGE_VERSION                  ?= ${AGENT_BUILD_NUMBER}
AGENT_VERSION                  ?= ${AGENT_BUILD_NUMBER}

NS                             ?= newrelic
REPO                           ?= infrastructure
IMAGE_NAME                     := ${NS}/${REPO}
CORE_IMAGE_NAME                := ${IMAGE_NAME}-core
BASE_IMAGE_NAME                := ${IMAGE_NAME}

AGENT_BIN                      ?= newrelic-infra
DOCKERFILE                     ?= $(CURDIR)/Dockerfile

# base variables
BASE_OHIS                      ?= ohis
DOCKER_VERSION                 ?= $(call get-nri-version,nri-docker)
FLEX_VERSION                   ?= $(call get-nri-version,nri-flex)

DOCKER_BUILD_BASE 				= docker build --no-cache \
								-t ${BASE_IMAGE_NAME}:${IMAGE_VERSION} \
								-t ${BASE_IMAGE_NAME}:latest \
								-t cf-registry.nr-ops.net/infrastructure/newrelic-infrastructure:${IMAGE_VERSION} \
								-t cf-registry.nr-ops.net/infrastructure/newrelic-infrastructure:latest \
								--build-arg image_version=${IMAGE_VERSION} \
								--build-arg agent_version=${AGENT_VERSION} \
								--build-arg version_file=VERSION \
								--build-arg agent_bin=${AGENT_BIN} \
								--build-arg nri_pkg_dir=${BASE_OHIS} \
								--build-arg nri_docker_version=${DOCKER_VERSION} \
								--build-arg nri_flex_version=${FLEX_VERSION} \
								--target base \
								-f ${DOCKERFILE} \
								${INFRASTRUCTURE_WORKSPACE}

workspace :
	@(mkdir -p ${PROJECT_WORKSPACE})

workspace/assets : workspace
	@(cp -r $(CURDIR)/assets ${PROJECT_WORKSPACE})
	@(echo ${IMAGE_VERSION} > ${PROJECT_WORKSPACE}/VERSION)

# Core
.PHONY : build/core
build/core : workspace/assets
build/core :
	@(docker build --no-cache \
	-t ${CORE_IMAGE_NAME}:${IMAGE_VERSION} \
	--build-arg image_version=${IMAGE_VERSION} \
	--build-arg agent_version=${AGENT_VERSION} \
	--build-arg version_file=VERSION \
	--build-arg agent_bin=${AGENT_BIN} \
	--target core \
	-f ${DOCKERFILE} \
	${INFRASTRUCTURE_WORKSPACE})

${NRI_PKG_DIR}:
	@(mkdir -p ${NRI_PKG_DIR})

## Base
#.PHONY : base/get-integrations
#base/get-integrations :
#	@($(MAKE)  && NRI_PKG_DIR="${PROJECT_WORKSPACE}/${BASE_OHIS}" $(MAKE) embed-integrations)

# Useful to get logged in Jenkins when things blow up
.PHONY : debug
debug :
	@(echo ${DOCKER_BUILD_BASE})

# Base
.PHONY : base/get-integrations
base/get-integrations : ${NRI_PKG_DIR}
base/get-integrations : get-integrations
base/get-integrations : embed-integrations

# Base
.PHONY : base/get-infra-agent
base/get-infra-agent : workspace
base/get-infra-agent:
	@(cp ${AGENT_BUILD_TARGET_DIR}/* ${PROJECT_WORKSPACE}/)

.PHONY : build/base
build/base : workspace/assets
build/base : base/get-integrations
build/base : base/get-infra-agent
	@(${DOCKER_BUILD_BASE})

# publish to docker hub
.PHONY : publish/base
publish/base : build/base
publish/base :
	@(docker push ${BASE_IMAGE_NAME}:${IMAGE_VERSION})
	@(docker push ${BASE_IMAGE_NAME}:latest)

.PHONY : lint
lint : lint/Dockerfile

.PHONY : lint/Dockerfile
lint/Dockerfile :
	@(docker run --rm -i hadolint/hadolint < ${DOCKERFILE})
