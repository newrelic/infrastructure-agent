.DEFAULT_GOAL := build/base

INFRASTRUCTURE_WORKSPACE	?= workspace
PROJECT_WORKSPACE		?= $(CURDIR)/$(INFRASTRUCTURE_WORKSPACE)

TARGET_DIR			?= $(PROJECT_WORKSPACE)/target/$(GOOS)_$(DOCKER_ARCH)
NRI_INTEGRATIONS_FILE		= $(CURDIR)/../embed/integrations.version
NRI_PKG_DIR			?= $(PROJECT_WORKSPACE)/$(BASE_OHIS)
AGENT_BUILD_TARGET_DIR		?= $(CURDIR)/../../dist
GOOS				?= linux
DOCKER_ARCH				?= amd64
DOCKER_BUILDX_CMD		?=
DOCKER_BUILDX_ARGS		?= --platform $(GOOS)/$(DOCKER_ARCH) --provenance=false --sbom=false
DOCKER_BUILDX_EXTRA_ARGS	?= --load
DOCKER_BUILD_TAG_PREFIX	?= build
DOCKER_TAG_LATEST	?= latest
USE_BUILDX			?= false
DOCKER_PUBLISH		?= false
FIPS                ?=

AGENT_ARCH ?= $(DOCKER_ARCH)

ifeq ($(AGENT_ARCH), arm)
	AGENT_ARCH := $(AGENT_ARCH)_6
endif

ifeq ($(AGENT_ARCH), arm64)
	AGENT_ARCH := $(AGENT_ARCH)_v8.0
endif

ifeq ($(AGENT_ARCH), amd64)
	AGENT_ARCH := $(AGENT_ARCH)_v1
endif

ifeq ($(DOCKER_PUBLISH), true)
	DOCKER_BUILDX_EXTRA_ARGS = --push
else
	DOCKER_BUILDX_EXTRA_ARGS = --load
endif

ifeq ($(USE_BUILDX), true)
	DOCKER_BUILDX_CMD	= buildx
	DOCKER_BUILDX_ARGS += $(DOCKER_BUILDX_EXTRA_ARGS)
else
	DOCKER_BUILDX_ARGS =
endif

OHI_ARCH ?= $(DOCKER_ARCH)

include ../embed/integrations.mk

AGENT_VERSION                  ?= 0.0.0
IMAGE_VERSION                  ?= $(AGENT_VERSION)

NS                             ?= newrelic
REPO                           ?= infrastructure${FIPS}
IMAGE_NAME                     ?= ${NS}/${REPO}
CORE_IMAGE_NAME                ?= ${IMAGE_NAME}-core
BASE_IMAGE_NAME                ?= ${IMAGE_NAME}
K8S_FWD_IMAGE_NAME			   ?= ${NS}/k8s-events-forwarder${FIPS}
FWD_IMAGE_NAME			   	   ?= ${NS}/nri-forwarder${FIPS}
DOCKER_IMAGE_NAME			   ?= ${BASE_IMAGE_NAME}

AGENT_BIN                      ?= newrelic-infra
DOCKERFILE                     ?= $(CURDIR)/Dockerfile

# base variables
BASE_OHIS                      ?= ohis
DOCKER_VERSION                 ?= $(call get-nri-version,nri-docker)
FLEX_VERSION                   ?= $(call get-nri-version,nri-flex)
PROMETHEUS_VERSION             ?= $(call get-nri-version,nri-prometheus)


DOCKER_TARGET					?= base
DOCKER_NO_CACHE					?= --no-cache
DOCKER_BUILD_BASE				?= docker \
								$(DOCKER_BUILDX_CMD) \
								build \
								$(DOCKER_BUILDX_ARGS) \
								$(DOCKER_NO_CACHE) \
								--pull \
								-t $(DOCKER_IMAGE_NAME):$(DOCKER_TAG_LATEST) \
								--build-arg image_version=$(IMAGE_VERSION) \
								--build-arg agent_version=$(AGENT_VERSION) \
								--build-arg version_file=VERSION \
								--build-arg agent_bin=$(AGENT_BIN) \
								--build-arg nri_pkg_dir=$(BASE_OHIS) \
								--build-arg nri_docker_version=$(DOCKER_VERSION) \
								--build-arg nri_flex_version=$(FLEX_VERSION) \
								--build-arg nri_prometheus_version=$(PROMETHEUS_VERSION) \
								--target $(DOCKER_TARGET) \
								-f $(DOCKERFILE) \
								$(INFRASTRUCTURE_WORKSPACE)

workspace :
	@(mkdir -p $(PROJECT_WORKSPACE))

workspace/assets : workspace
	@(cp -r $(CURDIR)/assets $(PROJECT_WORKSPACE))
	@(cp -r $(CURDIR)/../../assets/examples $(PROJECT_WORKSPACE)/assets/)
	@(echo $(IMAGE_VERSION) > $(PROJECT_WORKSPACE)/VERSION)

# Core
.PHONY : build/core
build/core : workspace/assets
build/core :
	@(docker build \
	$(DOCKER_NO_CACHE) \
	--pull \
	-t $(CORE_IMAGE_NAME):$(IMAGE_VERSION) \
	--build-arg image_version=$(IMAGE_VERSION) \
	--build-arg agent_version=$(AGENT_VERSION) \
	--build-arg version_file=VERSION \
	--build-arg agent_bin=$(AGENT_BIN) \
	--target core \
	-f $(DOCKERFILE) \
	$(INFRASTRUCTURE_WORKSPACE))

${NRI_PKG_DIR}:
	@(mkdir -p $(NRI_PKG_DIR))

# Useful to get logged in Jenkins when things blow up
.PHONY : debug
debug :
	@(echo $(DOCKER_BUILD_BASE))

# Base
.PHONY : base/get-integrations
base/get-integrations : ${NRI_PKG_DIR}
base/get-integrations : NRI_DOCKER_ARCH=$(DOCKER_ARCH)
base/get-integrations : NRI_PROMETHEUS_ARCH=$(DOCKER_ARCH)
base/get-integrations : NRI_FLEX_ARCH=$(DOCKER_ARCH)
base/get-integrations : get-integrations
base/get-integrations : embed-integrations

# Base
.PHONY : base/get-infra-agent
base/get-infra-agent : workspace
base/get-infra-agent:
	@(cp $(AGENT_BUILD_TARGET_DIR)/$(GOOS)-ctl-$(DOCKER_ARCH)_$(GOOS)_$(AGENT_ARCH)/* $(PROJECT_WORKSPACE)/)
	@(cp $(AGENT_BUILD_TARGET_DIR)/$(GOOS)-service-$(DOCKER_ARCH)_$(GOOS)_$(AGENT_ARCH)/* $(PROJECT_WORKSPACE)/)
	@(cp $(AGENT_BUILD_TARGET_DIR)/$(GOOS)-agent-$(DOCKER_ARCH)_$(GOOS)_$(AGENT_ARCH)/* $(PROJECT_WORKSPACE)/)

.PHONY : build/base
build/base : workspace/assets
build/base : base/get-integrations
build/base : base/get-infra-agent
build/base : DOCKER_TARGET=base
build/base :
	@($(DOCKER_BUILD_BASE))

.PHONY : build/forwarder
build/forwarder : workspace/assets
build/forwarder : base/get-infra-agent
build/forwarder : DOCKER_TARGET=forwarder
build/forwarder : DOCKER_IMAGE_NAME=$(FWD_IMAGE_NAME)
build/forwarder :
	@($(DOCKER_BUILD_BASE))

.PHONY : build/k8s-events-forwarder
build/k8s-events-forwarder : workspace/assets
build/k8s-events-forwarder : base/get-infra-agent
build/k8s-events-forwarder : DOCKER_TARGET=k8s-events-forwarder
build/k8s-events-forwarder : DOCKER_IMAGE_NAME=${K8S_FWD_IMAGE_NAME}
build/k8s-events-forwarder :
	@($(DOCKER_BUILD_BASE))

# publish to docker hub
.PHONY : publish/base
publish/base : build/base
publish/base :
	@printf '\n================================================================\n'
	@printf 'Target: publish/base\n'
	@printf 'TAG: $(BASE_IMAGE_NAME):$(IMAGE_VERSION)'
	@printf '\n================================================================\n'
	@(docker push $(BASE_IMAGE_NAME):$(IMAGE_VERSION))
	@(docker push $(BASE_IMAGE_NAME):latest)

.PHONY : lint
lint : lint/Dockerfile

.PHONY : lint/Dockerfile
lint/Dockerfile :
	@(docker run --rm -i hadolint/hadolint < $(DOCKERFILE))

.PHONY : clean
clean:
	@(rm -rf $(PROJECT_WORKSPACE))

buildTargetArch=$(lastword $(subst -, ,$1))
baseBuildTargetArch=$(lastword $(subst /, ,$1))

.PHONY : build/base-arm
build/base-arm :
	@($(MAKE) build/base/$(call buildTargetArch, $@))

.PHONY : build/base-arm64
build/base-arm64 :
	@($(MAKE) build/base/$(call buildTargetArch, $@))

.PHONY : build/base-amd64
build/base-amd64 :
	@($(MAKE) build/base/$(call buildTargetArch, $@))

# Helper target to build base images for each arch
.PHONY : build/base/%
build/base/% : export DOCKER_ARCH=$(call baseBuildTargetArch, $@)
build/base/% : export USE_BUILDX=true
build/base/% : export DOCKER_PUBLISH=true
build/base/% : export DOCKER_TAG_LATEST=$(DOCKER_BUILD_TAG_PREFIX)-$(DOCKER_ARCH)
build/base/% :
	@printf '\n================================================================\n'
	@printf 'Target: build/base-$(call baseBuildTargetArch, $@)\n'
	@printf 'DOCKER_TAG_LATEST: $(DOCKER_TAG_LATEST)'
	@printf '\n================================================================\n'
	@$(MAKE) build/base

.PHONY : build/forwarder-arm
build/forwarder-arm :
	@($(MAKE) build/forwarder/$(call buildTargetArch, $@))

.PHONY : build/forwarder-arm64
build/forwarder-arm64 :
	@($(MAKE) build/forwarder/$(call buildTargetArch, $@))

.PHONY : build/forwarder-amd64
build/forwarder-amd64 :
	@($(MAKE) build/forwarder/$(call buildTargetArch, $@))

# Helper target to build forwarder images for each arch
.PHONY : build/forwarder/%
build/forwarder/% : export DOCKER_ARCH=$(call baseBuildTargetArch, $@)
build/forwarder/% : export USE_BUILDX=true
build/forwarder/% : export DOCKER_PUBLISH=true
build/forwarder/% : export DOCKER_TAG_LATEST=$(DOCKER_BUILD_TAG_PREFIX)-$(DOCKER_ARCH)
build/forwarder/% :
	@printf '\n================================================================\n'
	@printf 'Target: build/forwarder-$(call baseBuildTargetArch, $@)\n'
	@printf 'DOCKER_TAG_LATEST: $(DOCKER_TAG_LATEST)'
	@printf '\n================================================================\n'
	@$(MAKE) build/forwarder

.PHONY : build/k8s-events-forwarder-arm
build/k8s-events-forwarder-arm :
	@($(MAKE) build/k8s-events-forwarder/$(call buildTargetArch, $@))

.PHONY : build/k8s-events-forwarder-arm64
build/k8s-events-forwarder-arm64 :
	@($(MAKE) build/k8s-events-forwarder/$(call buildTargetArch, $@))

.PHONY : build/k8s-events-forwarder-amd64
build/k8s-events-forwarder-amd64 :
	@($(MAKE) build/k8s-events-forwarder/$(call buildTargetArch, $@))

# Helper target to build k8s-events-forwarder images for each arch
.PHONY : build/k8s-events-forwarder/%
build/k8s-events-forwarder/% : export DOCKER_ARCH=$(call baseBuildTargetArch, $@)
build/k8s-events-forwarder/% : export USE_BUILDX=true
build/k8s-events-forwarder/% : export DOCKER_PUBLISH=true
build/k8s-events-forwarder/% : export DOCKER_TAG_LATEST=$(DOCKER_BUILD_TAG_PREFIX)-$(DOCKER_ARCH)
build/k8s-events-forwarder/% :
	@printf '\n================================================================\n'
	@printf 'Target: build/k8s-events-forwarder-$(call baseBuildTargetArch, $@)\n'
	@printf 'DOCKER_TAG_LATEST: $(DOCKER_TAG_LATEST)'
	@printf '\n================================================================\n'
	@$(MAKE) build/k8s-events-forwarder

# Creates multi acrh manifest for base image
.PHONY : publish/multi-arch-base-manifest
publish/multi-arch-base-manifest :
	@printf '\n================================================================\n'
	@printf 'Target: publish/base-manifest\n'
	@printf 'Image: $(DOCKER_IMAGE_NAME):$(IMAGE_VERSION)'
	@printf '\n================================================================\n'
ifeq ($(FIPS),)
	@(docker manifest create \
    $(DOCKER_IMAGE_NAME):$(IMAGE_VERSION) \
    	$(DOCKER_IMAGE_NAME):$(DOCKER_BUILD_TAG_PREFIX)-arm64 \
    	$(DOCKER_IMAGE_NAME):$(DOCKER_BUILD_TAG_PREFIX)-arm \
    	$(DOCKER_IMAGE_NAME):$(DOCKER_BUILD_TAG_PREFIX)-amd64)
else
	@(docker manifest create \
    $(DOCKER_IMAGE_NAME):$(IMAGE_VERSION) \
    	$(DOCKER_IMAGE_NAME):$(DOCKER_BUILD_TAG_PREFIX)-arm64 \
    	$(DOCKER_IMAGE_NAME):$(DOCKER_BUILD_TAG_PREFIX)-amd64)
endif
	@docker manifest push $(DOCKER_IMAGE_NAME):$(IMAGE_VERSION)


# [RC] Shortcut to build all supported multi arch bases and publish as RC
.PHONY : publish/multi-arch-base-rc
publish/multi-arch-base-rc : export IMAGE_VERSION=$(AGENT_VERSION)-rc
publish/multi-arch-base-rc : export DOCKER_BUILD_TAG_PREFIX=$(IMAGE_VERSION)
publish/multi-arch-base-rc : build/base-arm64
ifeq ($(FIPS),)
publish/multi-arch-base-rc : build/base-arm
endif
publish/multi-arch-base-rc : build/base-amd64
publish/multi-arch-base-rc : publish/multi-arch-base-manifest

# Shortcut to build all supported multi arch bases
.PHONY : publish/multi-arch-base-tag
publish/multi-arch-base-tag : export IMAGE_VERSION=$(AGENT_VERSION)
publish/multi-arch-base-tag : export DOCKER_BUILD_TAG_PREFIX=$(AGENT_VERSION)-rc
publish/multi-arch-base-tag : publish/multi-arch-base-manifest

# Shortcut to build all supported multi arch bases
.PHONY : publish/multi-arch-base-latest
publish/multi-arch-base-latest : export IMAGE_VERSION=latest
publish/multi-arch-base-latest : export DOCKER_BUILD_TAG_PREFIX=$(AGENT_VERSION)-rc
publish/multi-arch-base-latest : publish/multi-arch-base-manifest

# Creates multi acrh manifest for k8s-events-forwarder image
.PHONY : publish/multi-arch-k8s-events-forwarder-manifest
publish/multi-arch-k8s-events-forwarder-manifest : export DOCKER_IMAGE_NAME=$(K8S_FWD_IMAGE_NAME)
publish/multi-arch-k8s-events-forwarder-manifest :
	@printf '\n================================================================\n'
	@printf 'Target: publish/k8s-events-forwarder-manifest\n'
	@printf 'Image: $(DOCKER_IMAGE_NAME):$(IMAGE_VERSION)'
	@printf '\n================================================================\n'
ifeq ($(FIPS),)
	@(docker manifest create \
    $(DOCKER_IMAGE_NAME):$(IMAGE_VERSION) \
    	$(DOCKER_IMAGE_NAME):$(DOCKER_BUILD_TAG_PREFIX)-arm64 \
    	$(DOCKER_IMAGE_NAME):$(DOCKER_BUILD_TAG_PREFIX)-arm \
    	$(DOCKER_IMAGE_NAME):$(DOCKER_BUILD_TAG_PREFIX)-amd64)
else
	@(docker manifest create \
    $(DOCKER_IMAGE_NAME):$(IMAGE_VERSION) \
    	$(DOCKER_IMAGE_NAME):$(DOCKER_BUILD_TAG_PREFIX)-arm64 \
    	$(DOCKER_IMAGE_NAME):$(DOCKER_BUILD_TAG_PREFIX)-amd64)
endif
	@docker manifest push $(DOCKER_IMAGE_NAME):$(IMAGE_VERSION)

# [RC] Shortcut to build all supported multi arch k8s-events-forwarders and publish as RC
.PHONY : publish/multi-arch-k8s-events-forwarder-rc
publish/multi-arch-k8s-events-forwarder-rc : export IMAGE_VERSION=$(AGENT_VERSION)-rc
publish/multi-arch-k8s-events-forwarder-rc : export DOCKER_BUILD_TAG_PREFIX=$(IMAGE_VERSION)
publish/multi-arch-k8s-events-forwarder-rc : build/k8s-events-forwarder-arm64
ifeq ($(FIPS),)
publish/multi-arch-k8s-events-forwarder-rc : build/k8s-events-forwarder-arm
endif
publish/multi-arch-k8s-events-forwarder-rc : build/k8s-events-forwarder-amd64
publish/multi-arch-k8s-events-forwarder-rc : publish/multi-arch-k8s-events-forwarder-manifest

# Shortcut to build all supported multi arch k8s-events-forwarders
.PHONY : publish/multi-arch-k8s-events-forwarder-tag
publish/multi-arch-k8s-events-forwarder-tag : export IMAGE_VERSION=$(AGENT_VERSION)
publish/multi-arch-k8s-events-forwarder-tag : export DOCKER_BUILD_TAG_PREFIX=$(AGENT_VERSION)-rc
publish/multi-arch-k8s-events-forwarder-tag : publish/multi-arch-k8s-events-forwarder-manifest

# Shortcut to build all supported multi arch k8s-events-forwarders
.PHONY : publish/multi-arch-k8s-events-forwarder-latest
publish/multi-arch-k8s-events-forwarder-latest : export IMAGE_VERSION=latest
publish/multi-arch-k8s-events-forwarder-latest : export DOCKER_BUILD_TAG_PREFIX=$(AGENT_VERSION)-rc
publish/multi-arch-k8s-events-forwarder-latest : publish/multi-arch-k8s-events-forwarder-manifest

# Creates multi acrh manifest for forwarder image
.PHONY : publish/multi-arch-forwarder-manifest
publish/multi-arch-forwarder-manifest : export DOCKER_IMAGE_NAME=$(FWD_IMAGE_NAME)
publish/multi-arch-forwarder-manifest :
	@printf '\n================================================================\n'
	@printf 'Target: publish/forwarder-manifest\n'
	@printf 'Image: $(DOCKER_IMAGE_NAME):$(IMAGE_VERSION)'
	@printf '\n================================================================\n'
ifeq ($(FIPS),)
	@(docker manifest create \
    $(DOCKER_IMAGE_NAME):$(IMAGE_VERSION) \
    	$(DOCKER_IMAGE_NAME):$(DOCKER_BUILD_TAG_PREFIX)-arm64 \
    	$(DOCKER_IMAGE_NAME):$(DOCKER_BUILD_TAG_PREFIX)-arm \
    	$(DOCKER_IMAGE_NAME):$(DOCKER_BUILD_TAG_PREFIX)-amd64)
else
	@(docker manifest create \
    $(DOCKER_IMAGE_NAME):$(IMAGE_VERSION) \
    	$(DOCKER_IMAGE_NAME):$(DOCKER_BUILD_TAG_PREFIX)-arm64 \
    	$(DOCKER_IMAGE_NAME):$(DOCKER_BUILD_TAG_PREFIX)-amd64)
endif
	@docker manifest push $(DOCKER_IMAGE_NAME):$(IMAGE_VERSION)


# [RC] Shortcut to build all supported multi arch forwarders and publish as RC
.PHONY : publish/multi-arch-forwarder-rc
publish/multi-arch-forwarder-rc : export IMAGE_VERSION=$(AGENT_VERSION)-rc
publish/multi-arch-forwarder-rc : export DOCKER_BUILD_TAG_PREFIX=$(IMAGE_VERSION)
publish/multi-arch-forwarder-rc : build/forwarder-arm64
ifeq ($(FIPS),)
publish/multi-arch-forwarder-rc : build/forwarder-arm
endif
publish/multi-arch-forwarder-rc : build/forwarder-amd64
publish/multi-arch-forwarder-rc : publish/multi-arch-forwarder-manifest

# Shortcut to build all supported multi arch forwarders
.PHONY : publish/multi-arch-forwarder-tag
publish/multi-arch-forwarder-tag : export IMAGE_VERSION=$(AGENT_VERSION)
publish/multi-arch-forwarder-tag : export DOCKER_BUILD_TAG_PREFIX=$(AGENT_VERSION)-rc
publish/multi-arch-forwarder-tag : publish/multi-arch-forwarder-manifest

# Shortcut to build all supported multi arch forwarders
.PHONY : publish/multi-arch-forwarder-latest
publish/multi-arch-forwarder-latest : export IMAGE_VERSION=latest
publish/multi-arch-forwarder-latest : export DOCKER_BUILD_TAG_PREFIX=$(AGENT_VERSION)-rc
publish/multi-arch-forwarder-latest : publish/multi-arch-forwarder-manifest

#################################################################
# Windows Container Targets
#################################################################

# Include fluent-bit version parsing
include ../embed/fluent-bit.mk

# Get Fluent Bit versions for Windows
FB_VERSION_WINDOWS ?= $(call get-fb_version,windows,3)
FB_PLUGIN_VERSION_WINDOWS ?= $(call get-fb_version,windows,2)

WINDOWS_DOCKERFILE ?= $(CURDIR)/Dockerfile.windows
WINDOWS_IMAGE_NAME ?= ${NS}/${REPO}
WINDOWS_AGENT_BIN ?= newrelic-infra
WINDOWS_BASE_IMAGE ?= mcr.microsoft.com/windows/servercore:ltsc2022
WINDOWS_VERSION ?= ltsc2022
WINDOWS_ISOLATION ?=

# Windows-specific build command
DOCKER_BUILD_WINDOWS ?= docker \
	build \
	--no-cache \
	--pull \
	$(if $(WINDOWS_ISOLATION),--isolation=$(WINDOWS_ISOLATION),) \
	-t $(WINDOWS_IMAGE_NAME):$(DOCKER_TAG_LATEST) \
	--build-arg base_image=$(WINDOWS_BASE_IMAGE) \
	--build-arg image_version=$(IMAGE_VERSION) \
	--build-arg agent_version=$(AGENT_VERSION) \
	--build-arg version_file=VERSION \
	--build-arg agent_bin=$(WINDOWS_AGENT_BIN) \
	--build-arg fb_version=$(FB_VERSION_WINDOWS) \
	--build-arg fb_plugin_version=$(FB_PLUGIN_VERSION_WINDOWS) \
	-f $(WINDOWS_DOCKERFILE) \
	$(INFRASTRUCTURE_WORKSPACE)

# Get Windows agent binaries
.PHONY : windows/get-infra-agent
windows/get-infra-agent : workspace
windows/get-infra-agent :
	@(cp $(AGENT_BUILD_TARGET_DIR)/windows-ctl-amd64_windows_amd64_v1/*.exe $(PROJECT_WORKSPACE)/)
	@(cp $(AGENT_BUILD_TARGET_DIR)/windows-service-amd64_windows_amd64_v1/*.exe $(PROJECT_WORKSPACE)/)
	@(cp $(AGENT_BUILD_TARGET_DIR)/windows-agent-amd64_windows_amd64_v1/*.exe $(PROJECT_WORKSPACE)/)

# Build Windows base image (generic)
.PHONY : build/base-windows
build/base-windows : workspace/assets
build/base-windows : windows/get-infra-agent
build/base-windows : export GOOS=windows
build/base-windows : export DOCKER_ARCH=amd64
build/base-windows :
	@printf '\n================================================================\n'
	@printf 'Target: build/base-windows-$(WINDOWS_VERSION)\n'
	@printf 'Base: $(WINDOWS_BASE_IMAGE)\n'
	@printf 'Image: $(WINDOWS_IMAGE_NAME):$(DOCKER_TAG_LATEST)\n'
	@printf '================================================================\n'
	@($(DOCKER_BUILD_WINDOWS))

# Build Windows ltsc2022 image
.PHONY : build/base-windows-ltsc2022
build/base-windows-ltsc2022 : export WINDOWS_BASE_IMAGE=mcr.microsoft.com/windows/servercore:ltsc2022
build/base-windows-ltsc2022 : export WINDOWS_VERSION=ltsc2022
build/base-windows-ltsc2022 : export DOCKER_TAG_LATEST=$(DOCKER_BUILD_TAG_PREFIX)-windows-ltsc2022
build/base-windows-ltsc2022 :
	@($(MAKE) build/base-windows)

# Build Windows ltsc2019 image
.PHONY : build/base-windows-ltsc2019
build/base-windows-ltsc2019 : export WINDOWS_ISOLATION=hyperv
build/base-windows-ltsc2019 : export WINDOWS_BASE_IMAGE=mcr.microsoft.com/windows/servercore:ltsc2019
build/base-windows-ltsc2019 : export WINDOWS_VERSION=ltsc2019
build/base-windows-ltsc2019 : export DOCKER_TAG_LATEST=$(DOCKER_BUILD_TAG_PREFIX)-windows-ltsc2019
build/base-windows-ltsc2019 :
	@($(MAKE) build/base-windows)

# Publish Windows base RC (both versions)
.PHONY : publish/windows-base-rc
publish/windows-base-rc : export IMAGE_VERSION=$(AGENT_VERSION)-rc
publish/windows-base-rc : export DOCKER_BUILD_TAG_PREFIX=$(IMAGE_VERSION)
publish/windows-base-rc : build/base-windows-ltsc2022
publish/windows-base-rc : build/base-windows-ltsc2019
publish/windows-base-rc :
	@printf '\n================================================================\n'
	@printf 'Target: publish/windows-base-rc\n'
	@printf 'Pushing both ltsc2022 and ltsc2019 images'
	@printf '\n================================================================\n'
	@(docker push $(WINDOWS_IMAGE_NAME):$(IMAGE_VERSION)-windows-ltsc2022)
	@(docker push $(WINDOWS_IMAGE_NAME):$(IMAGE_VERSION)-windows-ltsc2019)

# Publish Windows base tag (both versions)
.PHONY : publish/windows-base-tag
publish/windows-base-tag : export IMAGE_VERSION=$(AGENT_VERSION)
publish/windows-base-tag :
	@printf '\n================================================================\n'
	@printf 'Target: publish/windows-base-tag\n'
	@printf 'Tagging and pushing versioned images'
	@printf '\n================================================================\n'
	@(docker tag $(WINDOWS_IMAGE_NAME):$(AGENT_VERSION)-rc-windows-ltsc2022 $(WINDOWS_IMAGE_NAME):$(IMAGE_VERSION)-ltsc2022)
	@(docker push $(WINDOWS_IMAGE_NAME):$(IMAGE_VERSION)-ltsc2022)
	@(docker tag $(WINDOWS_IMAGE_NAME):$(AGENT_VERSION)-rc-windows-ltsc2019 $(WINDOWS_IMAGE_NAME):$(IMAGE_VERSION)-ltsc2019)
	@(docker push $(WINDOWS_IMAGE_NAME):$(IMAGE_VERSION)-ltsc2019)

# Publish Windows base latest (both versions)
.PHONY : publish/windows-base-latest
publish/windows-base-latest :
	@printf '\n================================================================\n'
	@printf 'Target: publish/windows-base-latest\n'
	@printf 'Tagging and pushing latest images'
	@printf '\n================================================================\n'
	@(docker tag $(WINDOWS_IMAGE_NAME):$(AGENT_VERSION)-rc-windows-ltsc2022 $(WINDOWS_IMAGE_NAME):latest-ltsc2022)
	@(docker push $(WINDOWS_IMAGE_NAME):latest-ltsc2022)
	@(docker tag $(WINDOWS_IMAGE_NAME):$(AGENT_VERSION)-rc-windows-ltsc2019 $(WINDOWS_IMAGE_NAME):latest-ltsc2019)
	@(docker push $(WINDOWS_IMAGE_NAME):latest-ltsc2019)
	# Also tag ltsc2022 as the default 'latest'
	@(docker tag $(WINDOWS_IMAGE_NAME):$(AGENT_VERSION)-rc-windows-ltsc2022 $(WINDOWS_IMAGE_NAME):latest)
	@(docker push $(WINDOWS_IMAGE_NAME):latest)
