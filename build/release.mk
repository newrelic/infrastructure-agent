BUILD_DIR    := ./bin/
GORELEASER_VERSION := v0.155.0
GORELEASER_BIN ?= bin/goreleaser

bin:
	@mkdir -p $(BUILD_DIR)

$(GORELEASER_BIN): bin
	@echo "=== [$(GORELEASER_BIN)] Installing goreleaser $(GORELEASER_VERSION)"
	@(wget -qO /tmp/goreleaser.tar.gz https://github.com/goreleaser/goreleaser/releases/download/$(GORELEASER_VERSION)/goreleaser_$(OS_DOWNLOAD)_x86_64.tar.gz)
	@(tar -xf  /tmp/goreleaser.tar.gz -C bin/)
	@(rm -f /tmp/goreleaser.tar.gz)
	@echo "=== [$(GORELEASER_BIN)] goreleaser downloaded"

.PHONY : release/clean
release/clean:
	@echo "=== [release/clean] remove build metadata files"
	rm -fv $(CURDIR)/cmd/newrelic-infra/versioninfo.json
	rm -fv $(CURDIR)/cmd/newrelic-infra/resource.syso

.PHONY : release/deps
release/deps: $(GORELEASER_BIN)
	@echo "=== [release/deps] install goversioninfo"
	@go get github.com/josephspurrier/goversioninfo/cmd/goversioninfo

.PHONY : release/build
release/build: release/deps release/clean
	@echo "=== [release/build] build compiling all binaries"
	@$(GORELEASER_BIN) build --config $(CURDIR)/build/.goreleaser.yml --snapshot --rm-dist

.PHONY : release/pkg
release/pkg: release/deps release/clean
	@$(MAKE) get-integrations get-fluentbit-linux
	@echo "=== [release/build] PRE-RELEASE compiling all binaries, creating packages, archives"
	@$(GORELEASER_BIN) release --config $(CURDIR)/build/.goreleaser.yml --rm-dist $(PKG_FLAGS)

.PHONY : release/sign
release/sign:
	@echo "=== [release/sign] signing packages"
	@bash $(CURDIR)/build/sign.sh

.PHONY : release/publish
release/publish:
	@echo "=== [release/publish] publishing artifacts"
	@bash $(CURDIR)/build/upload_artifacts_gh.sh

.PHONY : release
release: release/pkg release/sign release/publish release/clean
	@echo "=== [release] full pre-release cycle complete for nix"

FILENAME_TARBALL = $(PROJECT_NAME)_linux_$(VERSION)_$(GOARCH).tar.gz
.PHONY: tarball-linux
tarball-linux:
	@$(MAKE) dist-for-os GOOS=linux #TODO remove this when goreleaser has all archs.
	$(eval TARBALL_BUILD_DIR := $(DIST_DIR)/newrelic-infra)
	@rm -rf $(TARBALL_BUILD_DIR)
	@mkdir -p $(TARBALL_BUILD_DIR)/
	@mkdir -p $(TARBALL_BUILD_DIR)/

	@mkdir -p $(TARBALL_BUILD_DIR)/etc/init_scripts/upstart
	@mkdir -p $(TARBALL_BUILD_DIR)/etc/init_scripts/systemd
	@mkdir -p $(TARBALL_BUILD_DIR)/etc/init_scripts/sysv

	# Add all overlay files to the package, to be placed in /var/db/newrelic-infra on installation
	@mkdir -p $(TARBALL_BUILD_DIR)/var/run/newrelic-infra
	@mkdir -p $(TARBALL_BUILD_DIR)/var/log/newrelic-infra
	@mkdir -p $(TARBALL_BUILD_DIR)/var/db/newrelic-infra
	@mkdir -p $(TARBALL_BUILD_DIR)/var/db/newrelic-infra/integrations.d
	@mkdir -p $(TARBALL_BUILD_DIR)/etc/newrelic-infra/integrations.d
	@mkdir -p $(TARBALL_BUILD_DIR)/var/db/newrelic-infra/custom-integrations
	@mkdir -p $(TARBALL_BUILD_DIR)/var/db/newrelic-infra/newrelic-integrations
	@mkdir -p $(TARBALL_BUILD_DIR)/usr/bin

	@cp -r $(TARGET_DIR)/bin/linux_$(GOARCH)/* $(TARBALL_BUILD_DIR)/usr/bin

	@cp -R $(PROJECT_WORKSPACE)/assets/examples/infrastructure/LICENSE.linux.txt $(TARBALL_BUILD_DIR)/var/db/newrelic-infra/LICENSE.txt

	@cp $(INCLUDE_BUILD_DIR)/package/upstart/newrelic-infra $(TARBALL_BUILD_DIR)/etc/init_scripts/upstart
	@cp $(INCLUDE_BUILD_DIR)/package/systemd/newrelic-infra.service $(TARBALL_BUILD_DIR)/etc/init_scripts/systemd
	@cp $(INCLUDE_BUILD_DIR)/package/sysv/deb/newrelic-infra $(TARBALL_BUILD_DIR)/etc/init_scripts/sysv
	@cp -r $(INCLUDE_BUILD_DIR)/package/binaries/linux/* $(TARBALL_BUILD_DIR)/

	@tar -czf $(DIST_DIR)/$(FILENAME_TARBALL) -C $(DIST_DIR) $(PROJECT_NAME)
	@rm -rf $(TARBALL_BUILD_DIR)

.PHONY: tarball-linux-all
tarball-linux-all:
	@for arch in "amd64" "386" "arm" "arm64" "mips" "mips64" "mips64le" "mipsle" "ppc64le" "s390x"  ; do \
		$(MAKE) tarball-linux GOARCH=$$arch || exit 1 ;\
	done

.PHONY: tarball-release
tarball-release: tarball-linux-all release/publish
	@echo "=== [release] releasing linux tarballs"

PRERELEASE := ${PRERELEASE}
ifneq ($(PRERELEASE), true)
	PKG_FLAGS := "--snapshot"
endif

OS := $(shell uname -s)
ifeq ($(OS), Darwin)
	OS_DOWNLOAD := "darwin"
	TAR := gtar
else
	OS_DOWNLOAD := "linux"
endif