BUILDER_IMG_TAG = infrastructure-agent-builder

.PHONY: ci/deps
ci/deps:
	@docker build -t $(BUILDER_IMG_TAG) -f $(CURDIR)/cicd/Dockerfile $(CURDIR)

.PHONY: ci/validate
ci/validate: ci/deps
	@docker run --rm -t \
			--name "infrastructure-agent-validate" \
			-v $(CURDIR):/go/src/github.com/newrelic/infrastructure-agent \
			-w /go/src/github.com/newrelic/infrastructure-agent \
			$(BUILDER_IMG_TAG) make validate

.PHONY: ci/snyk-test
ci/snyk-test:
	@docker run --rm -t \
			--name "infrastructure-agent-snyk-test" \
			-v $(CURDIR):/go/src/github.com/newrelic/infrastructure-agent \
			-w /go/src/github.com/newrelic/infrastructure-agent \
			-e SNYK_TOKEN \
			snyk/snyk:golang snyk test --severity-threshold=high

.PHONY: ci/test-coverage
ci/test-coverage: ci/deps
	@docker run --rm -t \
			--name "infrastructure-agent-test-coverage" \
			-v $(CURDIR):/go/src/github.com/newrelic/infrastructure-agent \
			-w /go/src/github.com/newrelic/infrastructure-agent \
			$(BUILDER_IMG_TAG) make test-coverage

.PHONY: ci/test-centos-5
ci/test-centos-5: ci/deps
	@docker run --rm -t \
			--name "infrastructure-agent-test-centos-5" \
			-v $(CURDIR):/go/src/github.com/newrelic/infrastructure-agent \
			-w /go/src/github.com/newrelic/infrastructure-agent \
			$(BUILDER_IMG_TAG) make go-get-go-1_9 test-centos-5
