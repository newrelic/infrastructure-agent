NRI_INTEGRATIONS_FILE	?= $(INCLUDE_BUILD_DIR)/embed/integrations.version
get-nri-version			= $(shell awk -F,v '/^$(1),v/ {print $$2}' ${NRI_INTEGRATIONS_FILE})

NRI_PKG_DIR				?= $(PKG_DIR)
PKG_DIR_BIN_OHIS    	?= $(NRI_PKG_DIR)/var/db/newrelic-infra/newrelic-integrations/bin

# Default OHI ARCH and OS
OHI_ARCH ?= amd64
OHI_OS   ?= linux

# nri-docker
NRI_DOCKER_VERSION ?= $(call get-nri-version,nri-docker)
NRI_DOCKER_ARCH    ?= $(OHI_ARCH)
NRI_DOCKER_OS      ?= $(OHI_OS)
NRI_DOCKER_URL     ?= https://download.newrelic.com/infrastructure_agent/binaries/$(NRI_DOCKER_OS)/$(NRI_DOCKER_ARCH)/nri-docker$(FIPS)_$(NRI_DOCKER_OS)_$(NRI_DOCKER_VERSION)_$(NRI_DOCKER_ARCH).tar.gz

# flex
NRI_FLEX_VERSION   ?= $(call get-nri-version,nri-flex)
NRI_FLEX_ARCH      ?= $(OHI_ARCH)
NRI_FLEX_OS        ?= $(OHI_OS)
NRI_FLEX_URL       ?= https://github.com/newrelic/nri-flex/releases/download/v$(NRI_FLEX_VERSION)/nri-flex$(FIPS)_$(NRI_FLEX_OS)_$(NRI_FLEX_VERSION)_$(NRI_FLEX_ARCH).tar.gz

# prometheus
NRI_PROMETHEUS_VERSION   ?= $(call get-nri-version,nri-prometheus)
NRI_PROMETHEUS_ARCH      ?= $(OHI_ARCH)
NRI_PROMETHEUS_OS        ?= $(OHI_OS)
NRI_PROMETHEUS_URL       ?= https://github.com/newrelic/nri-prometheus/releases/download/v$(NRI_PROMETHEUS_VERSION)/nri-prometheus$(FIPS)_$(NRI_PROMETHEUS_OS)_$(NRI_PROMETHEUS_VERSION)_$(NRI_PROMETHEUS_ARCH).tar.gz

.PHONY: get-integrations
get-integrations: get-nri-docker
get-integrations: get-nri-flex
get-integrations: get-nri-prometheus

.PHONY: get-nri-docker
get-nri-docker:
	@echo "NRI_DOCKER_ARCH=$(NRI_DOCKER_ARCH)"
	@printf '\n================================================================\n'
	@printf 'Target: download nri-docker for linux\n'
	@printf 'URL: $(NRI_DOCKER_URL)'
	@printf '\n================================================================\n'

	@rm -rf $(TARGET_DIR)/nridocker/$(NRI_DOCKER_ARCH)/
	@mkdir -p $(TARGET_DIR)/nridocker/$(NRI_DOCKER_ARCH)/
	@if curl --output /dev/null --silent --head --fail '$(NRI_DOCKER_URL)'; then \
		curl -L --silent '$(NRI_DOCKER_URL)' | tar xvz --no-same-owner -C $(TARGET_DIR)/nridocker/$(NRI_DOCKER_ARCH)/ ;\
	else \
	  echo 'nri-docker version $(NRI_DOCKER_VERSION) URL does not exist: $(NRI_DOCKER_URL)' ;\
	  exit 1 ;\
	fi

.PHONY: get-nri-flex
get-nri-flex:
	@printf '\n================================================================\n'
	@printf 'Target: download nri-flex for linux\n'
	@printf 'URL: $(NRI_FLEX_URL)'
	@printf '\n================================================================\n'

	@rm -rf $(TARGET_DIR)/nriflex/$(NRI_FLEX_ARCH)/
	@mkdir -p $(TARGET_DIR)/nriflex/$(NRI_FLEX_ARCH)/
	@if curl --output /dev/null --silent --head --fail '$(NRI_FLEX_URL)'; then \
		curl -L --silent '$(NRI_FLEX_URL)' | tar xvz --no-same-owner -C $(TARGET_DIR)/nriflex/$(NRI_FLEX_ARCH)/ ;\
	else \
	  echo 'nri-flex version $(NRI_FLEX_VERSION) URL does not exist: $(NRI_FLEX_URL)' ;\
	  exit 1 ;\
	fi

.PHONY: get-nri-prometheus
get-nri-prometheus:
	@printf '\n================================================================\n'
	@printf 'Target: download nri-prometheus for linux\n'
	@printf 'URL: $(NRI_PROMETHEUS_URL)'
	@printf '\n================================================================\n'

	@rm -rf $(TARGET_DIR)/nriprometheus/$(NRI_PROMETHEUS_ARCH)/
	@mkdir -p $(TARGET_DIR)/nriprometheus/$(NRI_PROMETHEUS_ARCH)/
	@if curl --output /dev/null --silent --head --fail '$(NRI_PROMETHEUS_URL)'; then \
		curl -L --silent '$(NRI_PROMETHEUS_URL)' | tar xvz --no-same-owner -C $(TARGET_DIR)/nriprometheus/$(NRI_PROMETHEUS_ARCH)/ ;\
	else \
	  echo 'nri-prometheus version $(NRI_PROMETHEUS_VERSION) URL does not exist: $(NRI_PROMETHEUS_URL)' ;\
	  exit 1 ;\
	fi
	
.PHONY: embed-nri-docker
embed-nri-docker:
	@echo "Embed nri-docker version: $(NRI_DOCKER_VERSION)"
	@cp -r $(TARGET_DIR)/nridocker/$(NRI_DOCKER_ARCH)/* $(NRI_PKG_DIR)

.PHONY: embed-nri-flex
embed-nri-flex:
	@echo "Embed nri-flex version: $(NRI_FLEX_VERSION)"
	@mkdir -p $(PKG_DIR_BIN_OHIS)
	@cp $(TARGET_DIR)/nriflex/$(NRI_FLEX_ARCH)/nri-flex $(PKG_DIR_BIN_OHIS)/

.PHONY: embed-nri-prometheus
embed-nri-prometheus:
	@echo "Embed nri-prometheus version: $(NRI_PROMETHEUS_VERSION)"
	@mkdir -p $(PKG_DIR_BIN_OHIS)
	@cp -r $(TARGET_DIR)/nriprometheus/$(NRI_PROMETHEUS_ARCH)/var $(NRI_PKG_DIR)

.PHONY: embed-integrations
embed-integrations: embed-nri-flex
embed-integrations: embed-nri-docker
embed-integrations: embed-nri-prometheus