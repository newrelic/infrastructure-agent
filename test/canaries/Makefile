# Include Ansible dependencies common installation strategy
include $(CURDIR)/test/ansible/Ansible.common

ANSIBLE_FOLDER := $(CURDIR)/test/canaries
ANSIBLE_FORKS ?= 20

.DEFAULT_GOAL := canaries

.PHONY: terraform-canaries
terraform-canaries: ansible/dependencies
ifndef ANSIBLE_INVENTORY
	$(error ANSIBLE_INVENTORY is not set)
endif

ifndef PREVIOUS_VERSION
	$(error PREVIOUS_VERSION is not set)
endif

ifndef VERSION
	$(error VERSION is not set)
endif

ifndef NR_LICENSE_KEY_CANARIES
	$(error NR_LICENSE_KEY_CANARIES is not set)
endif
	ansible-playbook -f $(ANSIBLE_FORKS) -i $(ANSIBLE_INVENTORY) "$(ANSIBLE_FOLDER)/deploy_canaries.yml" -e "current_version=$(VERSION) previous_version=$(PREVIOUS_VERSION) nr_license_key=$(NR_LICENSE_KEY_CANARIES) docker_username=$(DOCKER_USERNAME) docker_password=$(DOCKER_PASSWORD)"


.PHONY: clean
clean: ansible/clean
