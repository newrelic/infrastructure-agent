.DEFAULT_GOAL := build/base

INFRASTRUCTURE_WORKSPACE       ?= workspace
PROJECT_WORKSPACE              ?= $(CURDIR)/$(INFRASTRUCTURE_WORKSPACE)

include $(CURDIR)/scripts/embed_ohis.mk

# AGENT_BUILD_NUMBER comes from the jenkins job
IMAGE_VERSION                  ?= ${AGENT_BUILD_NUMBER}
AGENT_VERSION                  ?= ${AGENT_BUILD_NUMBER}

NS                             ?= newrelic
REPO                           ?= infrastructure
IMAGE_NAME                     := ${NS}/${REPO}
CORE_IMAGE_NAME                := ${IMAGE_NAME}-core
BASE_IMAGE_NAME                := ${IMAGE_NAME}

AGENT_BIN                      ?= newrelic-infra
DOCKERFILE                     ?= ./build/container/Dockerfile

# base variables
BASE_OHIS                      ?= ohis
DOCKER_VERSION                 ?= $(call get-nri-version,nri-docker)
FLEX_VERSION                   ?= $(call get-nri-version,nri-flex)

.PHONY : workspace
workspace :
	@(cp -r build/container/assets ${PROJECT_WORKSPACE})
	@(echo ${IMAGE_VERSION} > ${PROJECT_WORKSPACE}/VERSION)

# Useful to get logged in Jenkins when things blow up
.PHONY : debug
debug :
	@(echo "docker build --no-cache \
		-t ${BASE_IMAGE_NAME}:${IMAGE_VERSION} \
		--build-arg image_version=${IMAGE_VERSION} \
		--build-arg agent_version=${AGENT_VERSION} \
		--build-arg version_file=VERSION \
		--build-arg agent_bin=${AGENT_BIN} \
		--build-arg nri_pkg_dir=${BASE_OHIS} \
		--build-arg nri_docker_version=${DOCKER_VERSION} \
		--build-arg nri_flex_version=${FLEX_VERSION} \
		--target base \
		-f ${DOCKERFILE} \
		${INFRASTRUCTURE_WORKSPACE}")

# Core
.PHONY : build/core
build/core : workspace
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

# Base
.PHONY : base/get-integrations
base/get-integrations :
	@(mkdir -p ${PROJECT_WORKSPACE}/${BASE_OHIS})
	@($(MAKE) get-integrations && NRI_PKG_DIR="${PROJECT_WORKSPACE}/${BASE_OHIS}" $(MAKE) embed-integrations)

.PHONY : build/base
build/base : workspace
build/base : base/get-integrations
build/base :
	@(docker build --no-cache \
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
	${INFRASTRUCTURE_WORKSPACE})

# publish to container fabric
.PHONY : publish-cf/base
publish-cf/base : build/base
publish-cf/base :
	@(docker push cf-registry.nr-ops.net/infrastructure/newrelic-infrastructure:${IMAGE_VERSION})
	@(docker push cf-registry.nr-ops.net/infrastructure/newrelic-infrastructure:latest)

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
