SOURCE_FILES ?=./pkg/... ./cmd/... ./internal/... ./test/...
SOURCE_FILES_DIR ?= $(CURDIR)/pkg $(CURDIR)/cmd $(CURDIR)/internal $(CURDIR)/test
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

GOTOOLS ?=
GOTOOLS += github.com/jandelgado/gcov2lcov

GOARCH ?= amd64

LDFLAGS += -X main.buildVersion=$(VERSION)
LDFLAGS += -X main.gitCommit=${GIT_COMMIT}

TEST_FLAGS += -failfast
TEST_FLAGS += -race
TEST_FLAGS += -ldflags '-X github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration.minimumIntegrationIntervalOverride=2s'

export GO111MODULE := on
export PATH := $(PROJECT_WORKSPACE)/bin:$(PATH)

GO_TEST ?= test $(TEST_OPTIONS) $(TEST_FLAGS) $(SOURCE_FILES) -run $(TEST_PATTERN) -timeout=10m
GO_FMT 	?= gofmt -s -w -l $(SOURCE_FILES_DIR)

.PHONY: deps
deps:
	@printf '\n================================================================\n'
	@printf 'Target: go-get'
	@printf '\n================================================================\n'
	@$(GO_BIN) get $(GOTOOLS)
	@$(GO_BIN) mod tidy
	@$(GO_BIN) mod vendor
	@echo '[go-get] Done.'

.PHONY: test-coverage
test-coverage: TEST_FLAGS += -covermode=atomic -coverprofile=$(COVERAGE_FILE)
test-coverage: deps
	@printf '\n================================================================\n'
	@printf 'Target: test-coverage'
	@printf '\n================================================================\n'
	@echo '[test] Testing packages: $(SOURCE_FILES)'
	$(GO_BIN) $(GO_TEST)
	@echo '[test] Converting: $(COVERAGE_FILE) into lcov.info'
	@(gcov2lcov -infile=$(COVERAGE_FILE) -outfile=lcov.info)

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

.PHONY: dist/linux
dist/linux: $(ARCHS)

linux/%:
	@echo "building ${@}"
	@(arch=$$(echo ${@} | cut -d '/' -f2) ;\
	$(MAKE) dist-for-os GOOS=linux GOARCH=$$arch)

.PHONY: linux/harvest-tests
linux/harvest-tests: GOOS=linux
linux/harvest-tests: GOARCH=amd64
linux/harvest-tests: deps
	go test ./test/harvest -tags="harvest" -v

.PHONY: proxy-test
proxy-test:
	@docker-compose -f $(CURDIR)/test/proxy/docker-compose.yml up -d ; \
	go test --tags=proxytests ./test/proxy/; status=$$?; \
    docker-compose -f test/proxy/docker-compose.yml down; \
    exit $$status