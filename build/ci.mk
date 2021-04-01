BUILDER_IMG_TAG = infrastructure-agent-builder

.PHONY: ci/deps
ci/deps:
	@docker build -t $(BUILDER_IMG_TAG) -f $(CURDIR)/build/Dockerfile $(CURDIR)

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

.PHONY: ci/test-coverage
ci/test-coverage: ci/deps
	@docker run --rm -t \
			--name "infrastructure-agent-test-coverage" \
			-v $(CURDIR):/go/src/github.com/newrelic/infrastructure-agent \
			-w /go/src/github.com/newrelic/infrastructure-agent \
			$(BUILDER_IMG_TAG) make test-coverage

.PHONY: ci/snyk-test
ci/snyk-test:
	@docker run --rm -t \
			--name "infrastructure-agent-snyk-test" \
			-v $(CURDIR):/go/src/github.com/newrelic/infrastructure-agent \
			-w /go/src/github.com/newrelic/infrastructure-agent \
			-e SNYK_TOKEN \
			snyk/snyk:golang snyk test --severity-threshold=high

.PHONY : ci/prerelease/linux
ci/prerelease/linux:
	TARGET_OS=linux $(MAKE) ci/prerelease

.PHONY : ci/prerelease/macos
ci/prerelease/macos:
	TARGET_OS=macos $(MAKE) ci/prerelease

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
			$(BUILDER_IMG_TAG) make release-${TARGET_OS}
else
	@echo "===> infrastructure-agent ===  [ci/prerelease/linux] TAG env variable expected to be set"
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
			$(BUILDER_IMG_TAG) make release/pkg

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

