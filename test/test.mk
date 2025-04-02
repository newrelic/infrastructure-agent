PROVISION_HOST_PREFIX := $(shell whoami)-$(shell hostname)
AWS_ACCOUNT_ID = "971422713139"# OHAI
LIMIT ?= "testing_hosts"
ANSIBLE_FORKS ?= 5

ANSIBLE_INVENTORY_FOLDER ?= $(CURDIR)/test/automated/ansible

ifeq ($(origin ANSIBLE_INVENTORY_FILE), undefined)
  ANSIBLE_INVENTORY = $(CURDIR)/test/automated/ansible/inventory.ec2
else
  ANSIBLE_INVENTORY = $(ANSIBLE_INVENTORY_FOLDER)/$(ANSIBLE_INVENTORY_FILE)
endif

include $(CURDIR)/test/ansible/common.mk

# Allow running specific harvest tests based on regex (default to .*)
TESTS_TO_RUN_REGEXP ?= ".*"
.PHONY: test/automated/harvest
test/automated/harvest:
	AGENT_RUN_DIR=$(CURDIR) ANSIBLE_FORKS=$(ANSIBLE_FORKS) ANSIBLE_INVENTORY=$(ANSIBLE_INVENTORY) $(CURDIR)/test/harvest/ansible/harvest.sh

.PHONY: test/automated/install-requirements
test/automated/install-requirements:
	@ANSIBLE_DISPLAY_SKIPPED_HOSTS=NO ANSIBLE_DISPLAY_OK_HOSTS=NO ansible-playbook -f $(ANSIBLE_FORKS)  -i $(ANSIBLE_INVENTORY) --limit=$(LIMIT) $(CURDIR)/test/automated/ansible/install-requirements.yml


.PHONY: test/automated/packaging
test/automated/packaging: ansible/dependencies
ifndef NR_LICENSE_KEY
	@echo "NR_LICENSE_KEY variable must be provided for test/automated/packaging"
	exit 1
endif
ifndef NEW_RELIC_API_KEY
	@echo "NEW_RELIC_API_KEY variable must be provided for test/automated/packaging"
	exit 1
endif
ifndef NEW_RELIC_ACCOUNT_ID
	@echo "NEW_RELIC_ACCOUNT_ID variable must be provided for test/automated/packaging"
	exit 1
endif
	@ANSIBLE_DISPLAY_SKIPPED_HOSTS=NO ANSIBLE_DISPLAY_OK_HOSTS=NO ansible-playbook -f $(ANSIBLE_FORKS)  -i $(ANSIBLE_INVENTORY) --limit=$(LIMIT) -e nr_license_key=$(NR_LICENSE_KEY) -e nr_api_key=$(NEW_RELIC_API_KEY) -e nr_account_id=$(NEW_RELIC_ACCOUNT_ID) $(CURDIR)/test/packaging/ansible/test.yml

.PHONY: test/automated/packaging-docker
test/automated/packaging-docker:
ifndef AGENT_VERSION
	@echo "AGENT_VERSION variable must be provided for test/automated/packaging-docker"
	exit 1
else
	bash $(CURDIR)/test/packaging/docker.sh
endif
