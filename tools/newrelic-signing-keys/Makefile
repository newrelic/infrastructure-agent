IMAGE_VERSION ?= dev
IMAGE_NAME = newrelic-signing-keys
PKG_VERSION ?= 0.0.0

.PHONY: clean
clean:
	@rm -f pkg/*.deb

.PHONY: build-container
build-container:
	@docker build -t $(IMAGE_NAME):$(IMAGE_VERSION) .


.PHONY: build
build: clean build-container
	@docker run -v $(CURDIR)/pkg:/fpm/pkg $(IMAGE_NAME):$(IMAGE_VERSION) --version $(PKG_VERSION) .
