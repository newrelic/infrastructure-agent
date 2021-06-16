mkfile_path := $(abspath $(lastword $(MAKEFILE_LIST)))
current_dir := $(patsubst %/,%,$(dir $(mkfile_path)))

fb_version_file       ?= $(current_dir)/fluent-bit.version

# The second argument $(2) specifies which column to return
get-fb_version          = $(shell awk -v col=$(2) -F, '/^$(1),/ {print $$col}' ${fb_version_file})

NR_PLUGIN_VERSION_LINUX ?= $(call get-fb_version,linux,2)

FB_ARCH                 ?= amd64

NR_PLUGIN_URL            = https://github.com/newrelic/newrelic-fluent-bit-output/releases/download/v$(NR_PLUGIN_VERSION_LINUX)/out_newrelic-linux-$(FB_ARCH)-$(NR_PLUGIN_VERSION_LINUX).so

.PHONY: get-fluentbit-linux
get-fluentbit-linux:
	@printf '\n================================================================\n'
	@printf 'Target: download nr fluentbit plugin for linux'
	@printf '\n================================================================\n'

	@printf '\ndownload fluent-bit nr plugin\n'
	@rm -rf $(TARGET_DIR)/fluent-bit-plugin/$(FB_ARCH)/
	@mkdir -p $(TARGET_DIR)/fluent-bit-plugin/$(FB_ARCH)/
	@if curl --output /dev/null --silent --head --fail '$(NR_PLUGIN_URL)'; then \
		curl -L --silent '$(NR_PLUGIN_URL)' --output $(TARGET_DIR)/fluent-bit-plugin/$(FB_ARCH)/out_newrelic.so ;\
	else \
	  echo 'nr plugin version $(NR_PLUGIN_VERSION_LINUX) URL does not exist: $(NR_PLUGIN_URL)' ;\
	  exit 1 ;\
	fi
