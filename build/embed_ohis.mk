NRI_INTEGRATIONS_FILE	?= $(CURDIR)/build/nri-integrations
get-nri-version			= $(shell awk -F, '/^$(1),/ {print $$2}' ${NRI_INTEGRATIONS_FILE})

NRI_PKG_DIR				?= $(PKG_DIR)
PKG_DIR_BIN_OHIS    	?= $(NRI_PKG_DIR)/var/db/newrelic-infra/newrelic-integrations/bin

# nri-docker
NRI_DOCKER_VERSION ?= $(call get-nri-version,nri-docker)
NRI_DOCKER_ARCH    ?= amd64
NRI_DOCKER_URL     ?= https://download.newrelic.com/infrastructure_agent/binaries/linux/$(NRI_DOCKER_ARCH)/nri-docker_linux_$(NRI_DOCKER_VERSION)_$(NRI_DOCKER_ARCH).tar.gz

# flex
NRI_FLEX_VERSION   ?= $(call get-nri-version,nri-flex)
NRI_FLEX_OS        ?= Linux
NRI_FLEX_URL       ?= https://github.com/newrelic/nri-flex/releases/download/v$(NRI_FLEX_VERSION)/nri-flex_$(NRI_FLEX_VERSION)_$(NRI_FLEX_OS)_x86_64.tar.gz

# prometheus
NRI_PROMETHEUS_VERSION   ?= $(call get-nri-version,nri-prometheus)
NRI_PROMETHEUS_ARCH      ?= amd64
NRI_PROMETHEUS_OS        ?= linux
NRI_PROMETHEUS_URL       ?= https://github.com/newrelic/nri-prometheus/releases/download/v$(NRI_PROMETHEUS_VERSION)/nri-prometheus_$(NRI_PROMETHEUS_OS)_$(NRI_PROMETHEUS_ARCH)_$(NRI_PROMETHEUS_VERSION).tar.gz

.PHONY: get-integrations
get-integrations: get-nri-docker
get-integrations: get-nri-flex
get-integrations: get-nri-prometheus

.PHONY: get-nri-docker
get-nri-docker:
	rm -rf $(TARGET_DIR)/nridocker/
	mkdir -p $(TARGET_DIR)/nridocker/
	if curl --output /dev/null --silent --head --fail '$(NRI_DOCKER_URL)'; then \
		curl -L --silent '$(NRI_DOCKER_URL)' | tar xvz --no-same-owner -C $(TARGET_DIR)/nridocker/ ;\
	else \
	  echo 'nri-docker version $(NRI_DOCKER_VERSION) URL does not exist: $(NRI_DOCKER_URL)' ;\
	  exit 1 ;\
	fi

.PHONY: get-nri-flex
get-nri-flex:
	rm -rf $(TARGET_DIR)/nriflex/
	mkdir -p $(TARGET_DIR)/nriflex/
	if curl --output /dev/null --silent --head --fail '$(NRI_FLEX_URL)'; then \
		curl -L --silent '$(NRI_FLEX_URL)' | tar xvz --no-same-owner -C $(TARGET_DIR)/nriflex/ ;\
	else \
	  echo 'nri-flex version $(NRI_FLEX_VERSION) URL does not exist: $(NRI_FLEX_URL)' ;\
	  exit 1 ;\
	fi

.PHONY: get-nri-prometheus
get-nri-prometheus:
	rm -rf $(TARGET_DIR)/nriprometheus/
	mkdir -p $(TARGET_DIR)/nriprometheus/
	if curl --output /dev/null --silent --head --fail '$(NRI_PROMETHEUS_URL)'; then \
		curl -L --silent '$(NRI_PROMETHEUS_URL)' | tar xvz --no-same-owner -C $(TARGET_DIR)/nriprometheus/ ;\
	else \
	  echo 'nri-prometheus version $(NRI_PROMETHEUS_VERSION) URL does not exist: $(NRI_PROMETHEUS_URL)' ;\
	  exit 1 ;\
	fi


.PHONY: embed-nri-docker
embed-nri-docker:
	@echo "Embed nri-docker version: $(NRI_DOCKER_VERSION)"
	@cp -r $(TARGET_DIR)/nridocker/* $(NRI_PKG_DIR)

.PHONY: embed-nri-flex
embed-nri-flex:
	@echo "Embed nri-flex version: $(NRI_FLEX_VERSION)"
	@mkdir -p $(PKG_DIR_BIN_OHIS)
	@cp $(TARGET_DIR)/nriflex/nri-flex $(PKG_DIR_BIN_OHIS)/

.PHONY: embed-nri-prometheus
embed-nri-prometheus:
	@echo "Embed nri-prometheus version: $(NRI_PROMETHEUS_VERSION)"
	@mkdir -p $(PKG_DIR_BIN_OHIS)
	@cp -r $(TARGET_DIR)/nriprometheus/var $(NRI_PKG_DIR)

.PHONY: embed-integrations
embed-integrations: embed-nri-flex
embed-integrations: embed-nri-docker
embed-integrations: embed-nri-prometheus
