BUILD_DIR    := ./bin/
GORELEASER_VERSION := v0.143.0
GORELEASER_BIN ?= bin/goreleaser

bin:
	@mkdir -p $(BUILD_DIR)

$(GORELEASER_BIN): bin
	@echo "===> $(INTEGRATION) === [$(GORELEASER_BIN)] Installing goreleaser $(GORELEASER_VERSION)"
	@(wget -qO /tmp/goreleaser.tar.gz https://github.com/goreleaser/goreleaser/releases/download/$(GORELEASER_VERSION)/goreleaser_$(OS_DOWNLOAD)_x86_64.tar.gz)
	@(tar -xf  /tmp/goreleaser.tar.gz -C bin/)
	@(rm -f /tmp/goreleaser.tar.gz)
	@echo "===> $(INTEGRATION) === [$(GORELEASER_BIN)] goreleaser downloaded"

.PHONY : release/clean
release/clean:
	@echo "===> $(INTEGRATION) === [release/clean] remove build metadata files"
	rm -fv $(CURDIR)/cmd/newrelic-infra/versioninfo.json
	rm -fv $(CURDIR)/cmd/newrelic-infra/resource.syso

.PHONY : release/deps
release/deps: $(GORELEASER_BIN)
	@echo "===> $(INTEGRATION) === [release/deps] install goversioninfo"
	@go get github.com/josephspurrier/goversioninfo/cmd/goversioninfo

.PHONY : release/build
release/build: release/deps release/clean
#ifeq ($(PRERELEASE), true)
#	@echo "===> $(INTEGRATION) === [release/build] PRE-RELEASE compiling all binaries, creating packages, archives"
#	@$(GORELEASER_BIN) release --config $(CURDIR)/build/.goreleaser.yml --rm-dist
#else
	@echo "===> $(INTEGRATION) === [release/build] build compiling all binaries"
	@$(GORELEASER_BIN) build --config $(CURDIR)/build/.goreleaser.yml --snapshot --rm-dist
#endif

OS := $(shell uname -s)
ifeq ($(OS), Darwin)
	OS_DOWNLOAD := "darwin"
	TAR := gtar
else
	OS_DOWNLOAD := "linux"
endif