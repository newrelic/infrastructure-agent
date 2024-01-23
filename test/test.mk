include $(CURDIR)/test/ansible/Ansible.common

PROVISION_HOST_PREFIX := $(shell whoami)-$(shell hostname)
AWS_ACCOUNT_ID = "018789649883"# CAOS
LIMIT ?= "testing_hosts"
ANSIBLE_FORKS ?= 5

ANSIBLE_INVENTORY_FOLDER ?= $(CURDIR)/test/automated/ansible

ifeq ($(origin ANSIBLE_INVENTORY_FILE), undefined)
  ANSIBLE_INVENTORY = $(CURDIR)/test/automated/ansible/inventory.ec2
else
  ANSIBLE_INVENTORY = $(ANSIBLE_INVENTORY_FOLDER)/$(ANSIBLE_INVENTORY_FILE)
endif

.PHONY: test/automated/provision
test/automated/provision: validate-aws-credentials
ifndef PLATFORM
	@echo "PLATFORM variable must be provided for test/automated/provision"
	exit 1
endif
ifndef ANSIBLE_PASSWORD_WINDOWS
	@echo "ANSIBLE_PASSWORD_WINDOWS variable must be provided for test/automated/provision"
	exit 1
endif
	PROVISION_HOST_PREFIX=$(PROVISION_HOST_PREFIX) ANSIBLE_FORKS=$(ANSIBLE_FORKS) PLATFORM=$(PLATFORM) ANSIBLE_INVENTORY=$(ANSIBLE_INVENTORY) $(CURDIR)/test/automated/ansible/provision.sh

.PHONY: test/automated/termination
test/automated/termination: validate-aws-credentials
	ansible-playbook -i $(ANSIBLE_INVENTORY) $(CURDIR)/test/automated/ansible/termination.yml

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

.PHONY: validate-aws-credentials
validate-aws-credentials:
	@ACC_ID="$$(aws sts get-caller-identity --output text|awk '{print $$1}')"; \
	if [ "$${ACC_ID}" != "$(AWS_ACCOUNT_ID)" ]; then \
		echo "Invalid AWS account ID. Expected: $(AWS_ACCOUNT_ID), got: $${ACC_ID}."; \
		exit 1; \
	fi
