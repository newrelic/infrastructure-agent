BUILD_DIR			   := $(CURDIR)/bin
GORELEASER_VERSION	   := v0.155.0
GORELEASER_BIN		   ?= bin/goreleaser
GORELEASER_CONFIG_file ?= $(CURDIR)/build/.goreleaser.yml
GORELEASER_CONFIG	   ?= --config $(GORELEASER_CONFIG_file)
PKG_FLAGS              ?= --rm-dist

bin:
	@mkdir -p $(BUILD_DIR)

$(GORELEASER_BIN): bin
	@echo "=== [$(GORELEASER_BIN)] Installing goreleaser $(GORELEASER_VERSION)"
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
	$(GORELEASER_BIN) build $(GORELEASER_CONFIG) $(PKG_FLAGS)

.PHONY : release/pkg
release/pkg: release/deps release/clean
release/pkg: get-integrations get-fluentbit-linux
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

# snapshot replaces version tag for local builds
SNAPSHOT := ${SNAPSHOT}
ifeq ($(SNAPSHOT), true)
	PKG_FLAGS += --snapshot
endif

OS := $(shell uname -s)
ifeq ($(OS), Darwin)
	OS_DOWNLOAD := "darwin"
	TAR := gtar
else
	OS_DOWNLOAD := "linux"
endif