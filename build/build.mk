SOURCE_FILES ?=./pkg/... ./cmd/... ./internal/... ./test/...
SOURCE_FILES_DIR ?= $(CURDIR)/pkg $(CURDIR)/cmd $(CURDIR)/internal $(CURDIR)/test
TESTS_DATABIND = ./test/databind/...
TEST_PATTERN ?=.
TEST_OPTIONS ?=
ALL_PACKAGES ?= $(shell $(GO_BIN) list ./cmd/...)

MAIN_PACKAGES += ./cmd/$(PROJECT_NAME)
MAIN_PACKAGES += ./cmd/$(PROJECT_NAME)-service
MAIN_PACKAGES += ./cmd/$(PROJECT_NAME)-ctl

GIT_COMMIT = $(shell git rev-parse HEAD)
BUILD_DATE = $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_SHA    = $(shell git rev-parse --short HEAD)
GIT_TAG    = $(shell git describe --tags --abbrev=0 --exact-match 2>/dev/null)
GIT_DIRTY  = $(shell test -n "`git status --porcelain`" && echo "dirty" || echo "clean")

GOARCH ?= amd64

LDFLAGS += -X main.buildVersion=$(VERSION)
LDFLAGS += -X main.gitCommit=${GIT_COMMIT}
LDFLAGS += -X main.buildDate=${BUILD_DATE}

TEST_FLAGS += -failfast
TEST_FLAGS += -race
TEST_FLAGS += -ldflags '-X github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration.minimumIntegrationIntervalOverride=2s'

export GO111MODULE := on
export PATH := $(PROJECT_WORKSPACE)/bin:$(PATH)

GO_TEST ?= test $(TEST_OPTIONS) --tags=slow $(TEST_FLAGS) $(SOURCE_FILES) -run $(TEST_PATTERN) -timeout=30m
GO_TEST_DATABIND ?= test $(TEST_OPTIONS) --tags=databind $(TEST_FLAGS) $(TESTS_DATABIND) -run $(TEST_PATTERN) -timeout=30m
GO_FMT 	?= gofmt -s -w -l $(SOURCE_FILES_DIR)

.PHONY: unit-test-with-coverage
unit-test-with-coverage: TEST_FLAGS += -covermode=atomic -coverprofile=$(COVERAGE_FILE)
unit-test-with-coverage: deps
	@printf '\n================================================================\n'
	@printf 'Target: test-coverage'
	@printf '\n================================================================\n'
	@echo '[test] Testing packages: $(SOURCE_FILES)'
	$(GO_BIN) $(GO_TEST)

.PHONY: databind-test
databind-test: deps
	@printf '\n================================================================\n'
	@printf 'Target: databind-test'
	@printf '\n================================================================\n'
	@echo '[test] Testing packages: $(TESTS_DATABIND)'
	$(GO_BIN) $(GO_TEST_DATABIND)

.PHONY: test
test: deps test-only

.PHONY: test-only
test-only:
	@printf '\n================================================================\n'
	@printf 'Target: test'
	@printf '\n================================================================\n'
	@echo '[test] Testing packages: $(SOURCE_FILES)'
	$(GO_BIN) $(GO_TEST)

.PHONY: clean
clean: deps
	@printf '\n================================================================\n'
	@printf 'Target: clean'
	@printf '\n================================================================\n'
	@echo '[clean] Removing target directory and build scripts...'
	rm -rf $(TARGET_DIR)
	rm -rf $(DIST_DIR)
	@echo '[clean] Done.'

.PHONY: validate
validate:
	@printf '\n================================================================\n'
	@printf 'Target: validate'
	@printf '\n================================================================\n'
	@echo '[test] Validating packages: $(SOURCE_FILES)'
	@test -z "$(shell  $(GO_FMT) | tee /dev/stderr)"

.PHONY: lint
lint: install-tools
	@printf '\n================================================================\n'
	@printf 'Target: lint'
	@printf '\n================================================================\n'
	@echo '[lint] Lint packages: $(SOURCE_FILES)'
	@golangci-lint run

.PHONY: gofmt
gofmt:
	@printf '\n================================================================\n'
	@printf 'Target: gofmt'
	@printf '\n================================================================\n'
	@echo '[gofmt] Formatting go code...'
	$(GO_FMT)

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
	@for main_package in $(MAIN_PACKAGES);\
	do\
		echo "[dist] Creating executable: `basename $$main_package`";\
		$(GO_BIN) build -gcflags '-N -l' -ldflags '$(LDFLAGS)' -o $(DIST_DIR)/$(GOOS)-`basename $$main_package`_$(GOOS)_$(GOARCH)/`basename $$main_package` $$main_package || exit 1 ;\
	done

.PHONY: debug-for-os
debug-for-os:
	@printf '\n================================================================\n'
	@printf "[dist-for-os] Building for target for debugging GOOS=$(GOOS) GOARCH=$(GOARCH)"
	@printf '\n================================================================\n'
	@if [ -n "$(MAIN_PACKAGES)" ]; then \
		echo '[dist] Creating executables for main packages: $(MAIN_PACKAGES)' ;\
	else \
		echo '[dist] No executables to distribute - skipping dist target.' ;\
	fi
	@for main_package in $(MAIN_PACKAGES);\
	do\
		echo "[dist] Creating executable: `basename $$main_package`";\
		$(GO_BIN) build -buildvcs=false -gcflags "all=-N -l" -ldflags '$(LDFLAGS)' -o $(DIST_DIR)/$(GOOS)-`basename $$main_package`_$(GOOS)_$(GOARCH)/`basename $$main_package` $$main_package || exit 1 ;\
	done

.PHONY: dist/linux
dist/linux: $(ARCHS)

linux/%:
	@echo "building ${@}"
	@(arch=$$(echo ${@} | cut -d '/' -f2) ;\
	$(MAKE) dist-for-os GOOS=linux GOARCH=$$arch)

.PHONY: linux/harvest-tests
linux/harvest-tests: GOOS=linux
linux/harvest-tests: GOARCH=amd64
linux/harvest-tests: CGO_ENABLED=0
linux/harvest-tests: deps
	$(GO_BIN) test ./test/harvest -tags="harvest" -v

.PHONY: macos/harvest-tests
macos/harvest-tests: GOOS=darwin
macos/harvest-tests: GOARCH=amd64
macos/harvest-tests: CGO_ENABLED=1
macos/harvest-tests: deps
	$(GO_BIN) test ./test/harvest -tags="harvest" -v

.PHONY: build-harvest-tests
build-harvest-tests: CGO_ENABLED=0
build-harvest-tests: deps
	$(GO_BIN) test -c ./test/harvest -tags="harvest" -v

.PHONY: build-harvest-tests-fips
build-harvest-tests-fips: CGO_ENABLED=1
build-harvest-tests-fips: GOEXPERIMENT=boringcrypto
build-harvest-tests-fips: deps
	$(GO_BIN) test -c ./test/harvest -tags="harvest,fips" -v


.PHONY: proxy-test
proxy-test:
	@docker compose -f $(CURDIR)/test/proxy/docker-compose.yml up -d ; \
	$(GO_BIN) test --tags=proxytests ./test/proxy/; status=$$?; \
    docker compose -f test/proxy/docker-compose.yml down; \
    exit $$status
