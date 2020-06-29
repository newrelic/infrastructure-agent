SOURCE_FILES ?=./pkg/... ./cmd/... ./internal/...
TEST_PATTERN ?=.
TEST_OPTIONS ?=
ALL_PACKAGES ?= $(shell $(GO_BIN) list ./cmd/...)

MAIN_PACKAGES += ./cmd/$(PROJECT_NAME)
MAIN_PACKAGES += ./cmd/$(PROJECT_NAME)-service
MAIN_PACKAGES += ./cmd/$(PROJECT_NAME)-ctl

GIT_COMMIT = $(shell git rev-parse HEAD)
GIT_SHA    = $(shell git rev-parse --short HEAD)
GIT_TAG    = $(shell git describe --tags --abbrev=0 --exact-match 2>/dev/null)
GIT_DIRTY  = $(shell test -n "`git status --porcelain`" && echo "dirty" || echo "clean")

GOARCH ?= amd64

LDFLAGS += -X main.buildVersion=$(VERSION)
LDFLAGS += -X main.gitCommit=${GIT_COMMIT}

TEST_FLAGS += -failfast
TEST_FLAGS += -race

export GO111MODULE := on
export PATH := $(PROJECT_WORKSPACE)/bin:$(PATH)

GO_TEST ?= test $(TEST_OPTIONS) $(TEST_FLAGS) $(SOURCE_FILES) -run $(TEST_PATTERN) -timeout=5m

.PHONY: go-get-go-1_9
go-get-go-1_9:
	$(GO_BIN) get golang.org/dl/go1.9.4
	$(GO_BIN_1_9) download

.PHONY: go-get
go-get:
	@printf '\n================================================================\n'
	@printf 'Target: go-get'
	@printf '\n================================================================\n'
	$(GO_BIN) mod vendor
	@echo '[go-get] Done.'

.PHONY: test
test: go-get
	@printf '\n================================================================\n'
	@printf 'Target: test'
	@printf '\n================================================================\n'
	@echo '[test] Testing packages: $(SOURCE_FILES)'
	$(GO_BIN) $(GO_TEST)

.PHONY: clean
clean: go-get
	@printf '\n================================================================\n'
	@printf 'Target: clean'
	@printf '\n================================================================\n'
	@echo '[clean] Removing target directory and build scripts...'
	rm -rf $(TARGET_DIR)
	@echo '[clean] Done.'

.PHONY: compile
compile:
	@printf '\n================================================================\n'
	@printf 'Target: compile'
	@printf '\n================================================================\n'
	@echo '[compile] Building packages: $(ALL_PACKAGES)'
	$(GO_BIN) build -v $(ALL_PACKAGES)
	@echo '[compile] Done.'

.PHONY: dist
dist:
ifeq ($(UNAME_S),Darwin)
	@$(MAKE) dist-for-os GOOS=darwin GOARCH=$(GOARCH)
else ifeq ($(UNAME_S),Linux)
	@if [ ! -z "$(ARCHS)" ]; then \
		$(MAKE) -j 4 dist/linux; \
	else \
		$(MAKE) dist-for-os GOOS=linux GOARCH=$(GOARCH) ;\
	fi
endif

.PHONY: dist-for-os
dist-for-os:
	@printf '\n================================================================\n'
	@printf "[dist-for-os] Building for target GOOS=$(GOOS) GOARCH=$(GOARCH)"
	@printf '\n================================================================\n'
	@if [ -n "$(MAIN_PACKAGES)" ]; then \
		echo '[dist] Creating executables for main packages: $(MAIN_PACKAGES)' ;\
	else \
		echo '[dist] No executables to distribute - skipping dist target.' ;\
	fi
	@mkdir -p $(TARGET_DIR)/$(GOOS)_$(GOARCH)/bin
	@for main_package in $(MAIN_PACKAGES);\
	do\
		echo "[dist] Creating executable: `basename $$main_package`";\
		$(GO_BIN) build -gcflags '-N -l' -ldflags '$(LDFLAGS)' -o $(TARGET_DIR)/bin/$(GOOS)_$(GOARCH)/`basename $$main_package` $$main_package || exit 1 ;\
	done

.PHONY: dist/linux
dist/linux: $(ARCHS)

linux/%:
	@echo "building ${@}"
	@(arch=$$(echo ${@} | cut -d '/' -f2) ;\
	$(MAKE) dist-for-os GOOS=linux GOARCH=$$arch)

.PHONY: test-centos-5
test-centos-5: go-get
test-centos-5: TEST_FLAGS=
test-centos-5:
	@printf '\n================================================================\n'
	@printf 'Target: test-centos-5'
	@printf '\n================================================================\n'
	@echo '[test] Testing packages: $(SOURCE_FILES)'
	$(GO_BIN_1_9) $(GO_TEST)

.PHONY: compile-centos-5
compile-centos-5: go-get
	@printf '\n================================================================\n'
	@printf 'Target: compile-centos-5'
	@printf '\n================================================================\n'
	@echo '[compile] Building packages: $(ALL_PACKAGES)'
	$(GO_BIN_1_9) build -v $(ALL_PACKAGES)
	@echo '[compile] Done.'

.PHONY: dist-centos-5
dist-centos-5: GOOS=linux
dist-centos-5: GOARCH=amd64
dist-centos-5:
	@printf '\n================================================================\n'
	@printf '\nBONUS TARGET FOR CENTOS 5\n'
	@printf "[dist] Building for target GOOS=$(GOOS) GOARCH=$(GOARCH)"
	@printf '\n================================================================\n'
	@if [ -n "$(MAIN_PACKAGES)" ]; then \
		echo '[dist] Creating executables for main packages: $(MAIN_PACKAGES)' ;\
	else \
		echo '[dist] No executables to distribute - skipping dist target.' ;\
	fi
	@mkdir -p $(TARGET_DIR_CENTOS5)/bin
	@for main_package in $(MAIN_PACKAGES);\
	do\
		echo "[dist]   Creating executable: `basename $$main_package` in $(TARGET_DIR_CENTOS5)/`basename $$main_package`";\
		$(GO_BIN_1_9) build -ldflags '$(LDFLAGS)' -o $(TARGET_DIR_CENTOS5)/`basename $$main_package` $$main_package || exit 1 ;\
	done
