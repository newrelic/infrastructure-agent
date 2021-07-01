BUILD_DIR			   := $(CURDIR)/bin
GORELEASER_VERSION	   := v0.169.0
GORELEASER_BIN		   ?= bin/goreleaser
GORELEASER_CONFIG_LINUX ?= $(CURDIR)/build/.goreleaser_linux.yml
GORELEASER_CONFIG_MACOS ?= $(CURDIR)/build/.goreleaser_macos.yml
PKG_FLAGS              ?= --rm-dist

bin:
	@mkdir -p $(BUILD_DIR)

$(GORELEASER_BIN): bin
	@echo "=== [$(GORELEASER_BIN)] Installing goreleaser $(GORELEASER_VERSION)"
#TODO: Temporary goreleaser build for rpm trans scripts
	(wget -qO /tmp/goreleaser.tar.gz https://github.com/goreleaser/goreleaser/releases/download/$(GORELEASER_VERSION)/goreleaser_$(OS_DOWNLOAD)_x86_64.tar.gz)
	(tar -xf  /tmp/goreleaser.tar.gz -C bin/)
	(rm -f /tmp/goreleaser.tar.gz)
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
	$(GORELEASER_BIN) build --config $(GORELEASER_CONFIG_LINUX) $(PKG_FLAGS)

.PHONY : release/get-integrations-amd64
release/get-integrations-amd64:
	@OHI_ARCH=amd64 make get-integrations

.PHONY : release/get-integrations-amd64-macos
release/get-integrations-amd64-macos:
	@OHI_ARCH=amd64 OHI_OS=darwin make get-integrations

.PHONY : release/get-integrations-arm64
release/get-integrations-arm64:
	@OHI_ARCH=arm64 make get-integrations

.PHONY : release/get-integrations-arm
release/get-integrations-arm:
	@OHI_ARCH=arm make get-integrations

.PHONY : release/get-fluentbit-linux-amd64
release/get-fluentbit-linux-amd64:
	@FB_ARCH=amd64 make get-fluentbit-linux

.PHONY : release/get-fluentbit-linux-arm
release/get-fluentbit-linux-arm:
	@FB_ARCH=arm make get-fluentbit-linux

.PHONY : release/get-fluentbit-linux-arm64
release/get-fluentbit-linux-arm64:
	@FB_ARCH=arm64 make get-fluentbit-linux

.PHONY : release/pkg-linux
release/pkg-linux: release/deps release/clean
release/pkg-linux: release/get-integrations-amd64
release/pkg-linux: release/get-integrations-arm64
release/pkg-linux: release/get-integrations-arm
release/pkg-linux: release/get-fluentbit-linux-amd64
#release/pkg-linux: release/get-fluentbit-linux-arm
#release/pkg-linux: release/get-fluentbit-linux-arm64
	@echo "=== [release/pkg-linux] PRE-RELEASE compiling all binaries, creating packages, archives"
	$(GORELEASER_BIN) release --config $(GORELEASER_CONFIG_LINUX) $(PKG_FLAGS)

.PHONY : release/pkg-macos
release/pkg-macos: release/deps release/clean
#release/pkg-macos: release/get-integrations-amd64-macos NO ASSETS AVAILABLE FOR NOW
#release/pkg-macos: release/get-integrations-arm64
#release/pkg-macos: release/get-integrations-arm
#release/pkg-macos: release/get-fluentbit-macos-amd64
#release/pkg-macos: release/get-fluentbit-macos-arm
#release/pkg-macos: release/get-fluentbit-macos-arm64
	@echo "=== [release/pkg-macos] PRE-RELEASE compiling all binaries, creating packages, archives"
	$(GORELEASER_BIN) release --config $(GORELEASER_CONFIG_MACOS) $(PKG_FLAGS)

.PHONY : release/fix-tarballs-linux
release/fix-tarballs-linux:
	@echo "=== [release/fix-tarballs-linux] fixing tar.gz archives internal structure"
	@bash $(CURDIR)/build/fix_tarballs_linux.sh

.PHONY : release/fix-tarballs-macos
release/fix-tarballs-macos:
	@echo "=== [release/fix-tarballs-macos] fixing tar.gz archives internal structure"
	@bash $(CURDIR)/build/fix_tarballs_macos.sh

.PHONY : release/sign
release/sign:
	@echo "=== [release/sign] signing packages"
	@bash $(CURDIR)/build/sign.sh

.PHONY : release/publish
release/publish:
	@echo "=== [release/publish] publishing artifacts"
	@bash $(CURDIR)/build/upload_artifacts_gh.sh

.PHONY : release-linux
release-linux: release/pkg-linux release/fix-tarballs-linux release/sign release/publish
	@echo "=== [release-linux] full pre-release cycle complete for nix"

.PHONY : release-macos
release-macos: release/pkg-macos release/fix-tarballs-macos release/sign release/publish
	@echo "=== [release-macos] full pre-release cycle complete for macOS"

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
