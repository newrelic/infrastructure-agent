PROVISION_ALERTS_WORKSPACE	?= $(CURDIR)/tools/provision-alerts


.PHONY: provision-alerts-install-deps
provision-alerts-install-deps:
	@echo "installing dependencies..."
	@cd $(PROVISION_ALERTS_WORKSPACE) \
	&& go mod download

.PHONY: provision-alerts-tests
provision-alerts-tests: provision-alerts-install-deps
	cd $(PROVISION_ALERTS_WORKSPACE) \
	&& go test ./...

.PHONY: provision-alerts-build
provision-alerts-build:
	@echo "building..."
	@cd $(PROVISION_ALERTS_WORKSPACE) \
	 && mkdir -p bin \
	 && go build -o bin/provision-alerts main.go

.PHONY: provision-alerts
provision-alerts: PREFIX ?= "[auto]"
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
	@echo "provisioning alerts for $(PREFIX) : $(DISPLAY_NAME_PREVIOUS) <--> $(DISPLAY_NAME_CURRENT) "
	@$(PROVISION_ALERTS_WORKSPACE)/bin/provision-alerts \
					-display_name_current="$(DISPLAY_NAME_CURRENT)" \
					-display_name_previous="$(DISPLAY_NAME_PREVIOUS)" \
					-api_key="$(NR_API_KEY)" \
					-template="$(TEMPLATE)" \
					-prefix="$(PREFIX)"


.PHONY: provision-alerts-delete
provision-alerts-delete: validate-aws-credentials provision-alerts-install-deps provision-alerts-build
ifndef NR_API_KEY
	@echo "NR_API_KEY variable must be provided for \"make provision-alerts\""
	exit 1
endif
ifndef PREFIX
	@echo "PREFIX variable must be provided for \"make provision-alerts-delete\""
	exit 1
endif
	@echo "deleting alerts for $(PREFIX)"
	@$(PROVISION_ALERTS_WORKSPACE)/bin/provision-alerts \
					-delete="true" \
					-api_key="$(NR_API_KEY)" \
					-prefix="$(PREFIX)"