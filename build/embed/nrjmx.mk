####
# nrjmx.version - https://github.com/newrelic/nrjmx/releases
# Pins the nrjmx version bundled ONLY into the Agent Control OCI package.
# This is intentionally separate from integrations.version: nrjmx must not be
# embedded into the standalone infrastructure-agent release (deb/rpm/tarball/msi)
# or the Docker Hub container image, so none of the targets below are wired
# into get-integrations/embed-integrations/release/pkg-*/build/container.
####

mkfile_path := $(abspath $(lastword $(MAKEFILE_LIST)))
current_dir := $(patsubst %/,%,$(dir $(mkfile_path)))

NRJMX_VERSION_FILE ?= $(current_dir)/nrjmx.version
NRJMX_VERSION      ?= $(shell cat $(NRJMX_VERSION_FILE))

NRJMX_URL = https://github.com/newrelic/nrjmx/releases/download/$(NRJMX_VERSION)/nrjmx_linux_$(NRJMX_VERSION:v%=%)_noarch.tar.gz

.PHONY: get-nrjmx
get-nrjmx:
	@printf '\n================================================================\n'
	@printf 'Target: download nrjmx $(NRJMX_VERSION)\n'
	@printf 'URL: $(NRJMX_URL)'
	@printf '\n================================================================\n'

	@rm -rf $(TARGET_DIR)/nrjmx/
	@mkdir -p $(TARGET_DIR)/nrjmx/
	@if curl --output /dev/null --silent --head --fail '$(NRJMX_URL)'; then \
		curl -L --silent '$(NRJMX_URL)' | tar xz --no-same-owner -C $(TARGET_DIR)/nrjmx/ ;\
	else \
	  echo 'nrjmx version $(NRJMX_VERSION) URL does not exist: $(NRJMX_URL)' ;\
	  exit 1 ;\
	fi
