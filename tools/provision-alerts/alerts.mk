PROVISION_ALERTS_WORKSPACE	?= $(CURDIR)/tools/provision-alerts

.PHONY: provision-alerts/fetch-inventory
provision-alerts/fetch-inventory:
	@echo "fetching inventory..."
	rm $(CURDIR)/tools/provision-alerts/inventory.ec2 || true
	bash $(CURDIR)/tools/provision-alerts/fetch_inventory.sh $(TAG) > $(CURDIR)/tools/provision-alerts/inventory.ec2

.PHONY: provision-alerts/pre-release
provision-alerts/pre-release: validate-aws-credentials ec2-install-deps ec2-build provision-alerts/fetch-inventory
	@echo "creating alerts with inventory from $(CURDIR)/tools/provision-alerts/inventory.ec2"
	bash $(CURDIR)/tools/provision-alerts/create_alerts.sh $(CURDIR)/tools/provision-alerts/inventory.ec2 $(NR_API_KEY)
	@echo "creating alerts with inventory from $(CURDIR)/tools/provision-alerts/inventory.macos.ec2"
	bash $(CURDIR)/tools/provision-alerts/create_alerts.sh $(CURDIR)/tools/provision-alerts/inventory.macos.ec2 $(NR_API_KEY)

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
provision-alerts: provision-alerts-install-deps provision-alerts-build
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
provision-alerts-delete: provision-alerts-install-deps provision-alerts-build
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

.PHONY: provision-alerts-delete/pre-release
provision-alerts-delete/pre-release: PREFIX ?= "[pre-release]"
provision-alerts-delete/pre-release: provision-alerts-install-deps provision-alerts-build

ifndef NR_API_KEY
	@echo "NR_API_KEY variable must be provided for \"make provision-alerts\""
	exit 1
endif
	@echo "deleting alerts for $(PREFIX)"
	@$(PROVISION_ALERTS_WORKSPACE)/bin/provision-alerts \
					-delete="true" \
					-api_key="$(NR_API_KEY)" \
					-prefix="$(PREFIX)"