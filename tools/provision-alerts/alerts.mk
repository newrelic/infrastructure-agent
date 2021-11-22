PROVISION_ALERTS_WORKSPACE	?= $(CURDIR)/tools/provision-alerts

.PHONY: provision-alerts-install-deps
provision-alerts-install-deps:
	cd $(PROVISION_ALERTS_WORKSPACE) \
	&& go mod download

.PHONY: provision-alerts-tests
provision-alerts-tests: provision-alerts-install-deps
	cd $(PROVISION_ALERTS_WORKSPACE) \
	&& go test ./...

.PHONY: provision-alerts-build
provision-alerts-build:
	cd $(PROVISION_ALERTS_WORKSPACE) \
	 && mkdir -p bin \
	 && go build -o bin/provision-alerts main.go

.PHONY: provision-alerts
provision-alerts: validate-aws-credentials provision-alerts-install-deps provision-alerts-build
ifndef NR_API_KEY
	@echo "NR_API_KEY variable must be provided for \"make provision-alerts\""
	exit 1
endif
ifndef DISPLAY_NAME_CURRENT
	@echo "DISPLAY_NAME_CURRENT variable must be provided for \"make provision-alerts\""
	exit 1
endif
ifndef DISPLAY_NAME_PREVIOUS
	@echo "DISPLAY_NAME_PREVIOUS variable must be provided for \"make provision-alerts\""
	exit 1
endif
ifndef TEMPLATE
	@echo "TEMPLATE variable must be provided for \"make provision-alerts\""
	exit 1
endif

	$(PROVISION_ALERTS_WORKSPACE)/bin/provision-alerts \
					-display_name_current $(DISPLAY_NAME_CURRENT) \
					-display_name_previous $(DISPLAY_NAME_PREVIOUS) \
					-api_key $(NR_API_KEY) \
					-template $(TEMPLATE)


