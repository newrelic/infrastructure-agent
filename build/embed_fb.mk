mkfile_path := $(abspath $(lastword $(MAKEFILE_LIST)))
current_dir := $(patsubst %/,%,$(dir $(mkfile_path)))

fb_version_file       ?= $(current_dir)/nr_fb_version

# The second argument $(2) specifies which column to return
get-fb_version         = $(shell awk -v col=$(2) -F, '/^$(1),/ {print $$col}' ${fb_version_file})

NRFB_ARTIFACT_VERSION_LINUX  ?= $(call get-fb_version,linux,4)

NRI_FB_URL                    = https://download.newrelic.com/infrastructure_agent/logging/linux/nrfb-$(NRFB_ARTIFACT_VERSION_LINUX)-linux-amd64.tar.gz

.PHONY: get-fluentbit-linux
get-fluentbit-linux:
	@printf '\n================================================================\n'
	@printf 'Target: download nr fluentbit for linux'
	@printf '\n================================================================\n'

	rm -rf $(TARGET_DIR)/fluent-bit/
	mkdir -p $(TARGET_DIR)/fluent-bit/
	if curl --output /dev/null --silent --head --fail '$(NRI_FB_URL)'; then \
		curl -L --silent '$(NRI_FB_URL)' | tar xvz --no-same-owner -C $(TARGET_DIR)/fluent-bit ;\
	else \
	  echo 'nrfb version $(NRI_FB_URL) URL does not exist: $(NRI_FB_URL)' ;\
	  exit 1 ;\
	fi