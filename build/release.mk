BUILD_DIR			   := $(CURDIR)/bin
GORELEASER_VERSION	   := v0.155.0
GORELEASER_BIN		   ?= bin/goreleaser
GORELEASER_CONFIG_FILE ?= $(CURDIR)/build/.goreleaser.yml
GORELEASER_CONFIG	   ?= --config $(GORELEASER_CONFIG_FILE)
PKG_FLAGS              ?= --rm-dist

bin:
	@mkdir -p $(BUILD_DIR)

$(GORELEASER_BIN): bin
	@echo "=== [$(GORELEASER_BIN)] Installing goreleaser $(GORELEASER_VERSION)"
#TODO: Temporary goreleaser build for rpm trans scripts
	(wget -qO bin/goreleaser https://download.newrelic.com/infrastructure_agent/tools/goreleaser/0.0.1/goreleaser && chmod u+x bin/goreleaser)
#	(wget -qO /tmp/goreleaser.tar.gz https://github.com/goreleaser/goreleaser/releases/download/$(GORELEASER_VERSION)/goreleaser_$(OS_DOWNLOAD)_x86_64.tar.gz)
#	(tar -xf  /tmp/goreleaser.tar.gz -C bin/)
#	(rm -f /tmp/goreleaser.tar.gz)
	@echo "=== [$(GORELEASER_BIN)] goreleaser downloaded"

.PHONY : release/clean
release/clean:
	@echo "=== [release/clean] remove build metadata files"
	rm -fv $(CURDIR)/cmd/newrelic-infra/versioninfo.json
	rm -fv $(CURDIR)/cmd/newrelic-infra/resource.syso

.PHONY : release/deps
release/deps: $(GORELEASER_BIN)

.PHONY : release/build
release/build: release/deps release/clean
	@echo "=== [release/build] build compiling all binaries"
	$(GORELEASER_BIN) build $(GORELEASER_CONFIG) $(PKG_FLAGS)

.PHONY : release/get-integrations-amd64
release/get-integrations-amd64:
# trick to push makefile to execute same target twice with different params
	@OHI_ARCH=amd64 make get-integrations

.PHONY : release/get-integrations-arm64
release/get-integrations-arm64:
# trick to push makefile to execute same target twice with different params
	@OHI_ARCH=arm64 make get-integrations

.PHONY : release/get-integrations-arm
release/get-integrations-arm:
# trick to push makefile to execute same target twice with different params
	@OHI_ARCH=arm make get-integrations

.PHONY : release/get-fluentbit-linux-amd64
release/get-fluentbit-linux-amd64:
# trick to push makefile to execute same target twice with different params
	@FB_ARCH=amd64 make get-fluentbit-linux

.PHONY : release/get-fluentbit-linux-arm
release/get-fluentbit-linux-arm:
# trick to push makefile to execute same target twice with different params
	@FB_ARCH=arm make get-fluentbit-linux

.PHONY : release/get-fluentbit-linux-arm64
release/get-fluentbit-linux-arm64:
# trick to push makefile to execute same target twice with different params
	@FB_ARCH=arm64 make get-fluentbit-linux

.PHONY : release/pkg
release/pkg: release/deps release/clean
release/pkg: release/get-integrations-amd64
release/pkg: release/get-integrations-arm64
release/pkg: release/get-integrations-arm
release/pkg: release/get-fluentbit-linux-amd64
release/pkg: release/get-fluentbit-linux-arm
release/pkg: release/get-fluentbit-linux-arm64
	@echo "=== [release/build] PRE-RELEASE compiling all binaries, creating packages, archives"
	$(GORELEASER_BIN) release $(GORELEASER_CONFIG) $(PKG_FLAGS)

.PHONY : release/fix-tarballs
release/fix-tarballs:
	@echo "=== [release/fix-tarballs] fixing tar.gz archives internal structure"
	@bash $(CURDIR)/build/fix_tarballs.sh

.PHONY : release/sign
release/sign:
	@echo "=== [release/sign] signing packages"
	@bash $(CURDIR)/build/sign.sh

.PHONY : release/publish
release/publish:
	@echo "=== [release/publish] publishing artifacts"
	@bash $(CURDIR)/build/upload_artifacts_gh.sh

.PHONY : release
release: release/pkg release/fix-tarballs release/sign release/publish
	@echo "=== [release] full pre-release cycle complete for nix"

ifndef SNAPSHOT
	$(error SNAPSHOT is undefined)
endif

# snapshot replaces version tag for local builds, also --skip-validate to avoid git errors
SNAPSHOT := ${SNAPSHOT}
ifeq ($(SNAPSHOT), true)
	PKG_FLAGS += --snapshot --skip-validate
endif

OS := $(shell uname -s)
ifeq ($(OS), Darwin)
	OS_DOWNLOAD := "darwin"
	TAR := gtar
else
	OS_DOWNLOAD := "linux"
endif
