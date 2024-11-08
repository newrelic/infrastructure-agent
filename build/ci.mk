BUILDER_IMG_TAG = infrastructure-agent-builder
BUILDER_IMG_TAG_FIPS = infrastructure-agent-builder-fips
MODE=?

.PHONY: ci/deps
ci/deps:GH_ARCH ?= amd64
ci/deps:
	@docker build -t $(BUILDER_IMG_TAG) --build-arg GH_ARCH=$(GH_ARCH) -f $(CURDIR)/build/Dockerfile $(CURDIR)

.PHONY: ci/validate
ci/validate: ci/deps
	@docker run --rm -t \
			--name "infrastructure-agent-validate" \
			-v $(CURDIR):/go/src/github.com/newrelic/infrastructure-agent \
			-w /go/src/github.com/newrelic/infrastructure-agent \
			$(BUILDER_IMG_TAG) make validate

.PHONY : ci/build
ci/build: ci/deps
ifdef TAG
	@docker run --rm -t \
			--name "infrastructure-agent-test-build" \
			-v $(CURDIR):/go/src/github.com/newrelic/infrastructure-agent \
			-w /go/src/github.com/newrelic/infrastructure-agent \
			-e TAG \
			-e SNAPSHOT=true \
			$(BUILDER_IMG_TAG) make release/build
else
	@echo "===  [ci/build] TAG env variable expected to be set"
	exit 1
endif

.PHONY: ci/unit-test
ci/unit-test: ci/deps
	@docker run --rm -t \
			--name "infrastructure-agent-test-coverage" \
			-v $(CURDIR):/go/src/github.com/newrelic/infrastructure-agent \
			-w /go/src/github.com/newrelic/infrastructure-agent \
			--env CI \
			$(BUILDER_IMG_TAG) make unit-test-with-coverage

.PHONY: ci/snyk-test
ci/snyk-test:
	@docker run --rm -t \
			--name "infrastructure-agent-snyk-test" \
			-v $(CURDIR):/go/src/github.com/newrelic/infrastructure-agent \
			-w /go/src/github.com/newrelic/infrastructure-agent \
			-e SNYK_TOKEN \
			-e GOFLAGS="-buildvcs=false" \
			snyk/snyk:golang snyk test --severity-threshold=high

.PHONY: ci/tools-test
ci/tools-test: ci/deps
	@docker run --rm -t \
			--name "infrastructure-agent-test-coverage" \
			-v $(CURDIR):/go/src/github.com/newrelic/infrastructure-agent \
			-w /go/src/github.com/newrelic/infrastructure-agent \
			$(BUILDER_IMG_TAG) make tools-test

.PHONY : ci/prerelease/linux
ci/prerelease/linux:
	TARGET_OS=linux $(MAKE) ci/prerelease

.PHONY : ci/prerelease/linux-fips
ci/prerelease/linux-fips:
	TARGET_OS=linux-fips $(MAKE) ci/prerelease

.PHONY : ci/prerelease/linux-amd64
ci/prerelease/linux-amd64:
	TARGET_OS=linux-amd64 $(MAKE) ci/prerelease

.PHONY : ci/prerelease/linux-arm
ci/prerelease/linux-arm:
	TARGET_OS=linux-arm $(MAKE) ci/prerelease

.PHONY : ci/prerelease/linux-arm64
ci/prerelease/linux-arm64:
	TARGET_OS=linux-arm64 $(MAKE) ci/prerelease

.PHONY : ci/prerelease/linux-legacy
ci/prerelease/linux-legacy:
	TARGET_OS=linux-legacy $(MAKE) ci/prerelease

.PHONY : ci/prerelease/linux-for-docker
ci/prerelease/linux-for-docker:
	TARGET_OS=linux-for-docker $(MAKE) ci/prerelease


.PHONY : ci/prerelease/macos
ci/prerelease/macos:
ifdef TAG
	PRERELEASE=true \
	SNAPSHOT=false \
		$(MAKE) release-macos
else
	@echo "===> infrastructure-agent ===  [ci/prerelease/macos] TAG env variable expected to be set"
	exit 1
endif

.PHONY : ci/prerelease
ci/prerelease: ci/deps
ifdef TAG
	@docker run --rm -t \
			--name "infrastructure-agent-prerelease" \
			-v $(CURDIR):/go/src/github.com/newrelic/infrastructure-agent \
            -w /go/src/github.com/newrelic/infrastructure-agent \
			-e PRERELEASE=true \
			-e GITHUB_TOKEN \
			-e TAG \
			-e GPG_MAIL \
			-e GPG_PASSPHRASE \
			-e GPG_PRIVATE_KEY_BASE64 \
			-e SNAPSHOT=false \
			-e FIPS \
			$(BUILDER_IMG_TAG) make release-${TARGET_OS}
else
	@echo "===> infrastructure-agent ===  [ci/prerelease/linux] TAG env variable expected to be set"
	exit 1
endif

.PHONY : ci/prerelease-publish
ci/prerelease-publish: ci/deps
ifdef TAG
	# avoid container network errors in GHA runners
	@echo "Creating iptables rule to drop invalid packages"
	@$(shell @sudo iptables -D INPUT -i eth0 -m state --state INVALID -j DROP 2>/dev/null)
	@sudo iptables -A INPUT -i eth0 -m state --state INVALID -j DROP

	@docker run --rm -t \
			--name "infrastructure-agent-prerelease-publish" \
			-v $(CURDIR):/go/src/github.com/newrelic/infrastructure-agent \
            -w /go/src/github.com/newrelic/infrastructure-agent \
			-e GITHUB_TOKEN \
			-e TAG \
			$(BUILDER_IMG_TAG) make release-publish

else
	@echo "===> infrastructure-agent ===  [ci/prerelease-publish] TAG env variable expected to be set"
	exit 1
endif

.PHONY : dev/release/pkg
dev/release/pkg: ci/deps
	@docker run --rm -t \
			--name "infrastructure-agent-prerelease" \
			-v $(CURDIR):/go/src/github.com/newrelic/infrastructure-agent \
            -w /go/src/github.com/newrelic/infrastructure-agent \
			-e TAG=0.0.0 \
			-e SNAPSHOT=true \
			$(BUILDER_IMG_TAG) make release/pkg-linux

.PHONY : ci/validate-aws-credentials
ci/validate-aws-credentials:
ifndef AWS_PROFILE
	@echo "AWS_PROFILE variable must be provided"
	exit 1
endif
ifndef AWS_REGION
	@echo "AWS_REGION variable must be provided"
	exit 1
endif

.PHONY : ci/sync-s3/staging
ci/sync-s3/staging: ci/validate-aws-credentials
	@aws s3 rm --recursive s3://nr-downloads-ohai-staging/infrastructure_agent
	@aws s3 cp --recursive --exclude '*/infrastructure_agent/beta/*' --exclude '*/infrastructure_agent/test/*' --exclude '*/newrelic-infra.repo' s3://nr-downloads-main/infrastructure_agent/ s3://nr-downloads-ohai-staging/infrastructure_agent/

.PHONY : ci/sync-s3/testing
ci/sync-s3/testing: ci/validate-aws-credentials
	@aws s3 rm --recursive s3://nr-downloads-ohai-testing/infrastructure_agent
	@aws s3 cp --recursive --exclude '*/infrastructure_agent/beta/*' --exclude '*/infrastructure_agent/test/*' --exclude '*/newrelic-infra.repo' s3://nr-downloads-main/infrastructure_agent/ s3://nr-downloads-ohai-testing/infrastructure_agent/

.PHONY : ci/sync-s3/preview-staging
ci/sync-s3/preview-staging: ci/validate-aws-credentials
	@aws s3 rm --recursive s3://nr-downloads-ohai-staging/preview
	@aws s3 cp --recursive --exclude '*/newrelic-infra.repo' s3://nr-downloads-main/preview/ s3://nr-downloads-ohai-staging/preview/

.PHONY: ci/third-party-notices-check
ci/third-party-notices-check: ci/deps
	@docker run --rm -t \
			--name "infrastructure-agent-third-party-notices" \
			-v $(CURDIR):/go/src/github.com/newrelic/infrastructure-agent \
			-w /go/src/github.com/newrelic/infrastructure-agent \
			$(BUILDER_IMG_TAG) make third-party-notices-check

.PHONY: ci/license-header-check
ci/license-header-check: ci/deps
	@docker run --rm -t \
			--name "infrastructure-agent-header-licenses" \
			-v $(CURDIR):/go/src/github.com/newrelic/infrastructure-agent \
			-w /go/src/github.com/newrelic/infrastructure-agent \
			$(BUILDER_IMG_TAG) make checklicense
