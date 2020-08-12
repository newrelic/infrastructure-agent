.DEFAULT_GOAL := build/base

INFRASTRUCTURE_WORKSPACE       ?= workspace
PROJECT_WORKSPACE              ?= $(CURDIR)/$(INFRASTRUCTURE_WORKSPACE)

TARGET_DIR						= $(PROJECT_WORKSPACE)/target
NRI_INTEGRATIONS_FILE			= $(CURDIR)/../../build/nri-integrations
NRI_PKG_DIR						?= ${PROJECT_WORKSPACE}/${BASE_OHIS}
AGENT_BUILD_TARGET_DIR			?= $(CURDIR)/../../target/bin/linux_amd64

include ../../build/embed_ohis.mk

# AGENT_BUILD_NUMBER comes from the jenkins job
AGENT_BUILD_NUMBER			   ?= 0.0
IMAGE_VERSION                  ?= ${AGENT_BUILD_NUMBER}
AGENT_VERSION                  ?= ${AGENT_BUILD_NUMBER}

NS                             ?= newrelic
REPO                           ?= infrastructure
IMAGE_NAME                     ?= ${NS}/${REPO}
CORE_IMAGE_NAME                ?= ${IMAGE_NAME}-core
BASE_IMAGE_NAME                ?= ${IMAGE_NAME}
K8S_FWD_IMAGE_NAME			   ?= ${NS}/k8s-events-forwarder
FWD_IMAGE_NAME			   	   ?= ${NS}/nri-forwarder
DOCKER_IMAGE_NAME			   ?= ${BASE_IMAGE_NAME}

AGENT_BIN                      ?= newrelic-infra
DOCKERFILE                     ?= $(CURDIR)/Dockerfile

# base variables
BASE_OHIS                      ?= ohis
DOCKER_VERSION                 ?= $(call get-nri-version,nri-docker)
FLEX_VERSION                   ?= $(call get-nri-version,nri-flex)

DOCKER_TARGET					?= base
DOCKER_NO_CACHE					?= --no-cache
DOCKER_BUILD_BASE				?= docker build \
								${DOCKER_NO_CACHE} \
								--pull \
								-t ${DOCKER_IMAGE_NAME}:${IMAGE_VERSION} \
								-t ${DOCKER_IMAGE_NAME}:latest \
								--build-arg image_version=${IMAGE_VERSION} \
								--build-arg agent_version=${AGENT_VERSION} \
								--build-arg version_file=VERSION \
								--build-arg agent_bin=${AGENT_BIN} \
								--build-arg nri_pkg_dir=${BASE_OHIS} \
								--build-arg nri_docker_version=${DOCKER_VERSION} \
								--build-arg nri_flex_version=${FLEX_VERSION} \
								--target ${DOCKER_TARGET} \
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
	@(docker build \
	${DOCKER_NO_CACHE} \
	--pull \
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
build/base : DOCKER_TARGET=base
build/base :
	@(${DOCKER_BUILD_BASE})

.PHONY : build/forwarder
build/forwarder : workspace/assets
build/forwarder : base/get-infra-agent
build/forwarder : DOCKER_TARGET=forwarder
build/forwarder : DOCKER_IMAGE_NAME=${FWD_IMAGE_NAME}
build/forwarder :
	@(${DOCKER_BUILD_BASE})

.PHONY : build/k8s-events-forwarder
build/k8s-events-forwarder : workspace/assets
build/k8s-events-forwarder : base/get-infra-agent
build/k8s-events-forwarder : DOCKER_TARGET=k8s-events-forwarder
build/k8s-events-forwarder : DOCKER_IMAGE_NAME=${K8S_FWD_IMAGE_NAME}
build/k8s-events-forwarder :
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

.PHONY : clean
clean:
	@(rm -rf ${PROJECT_WORKSPACE})
