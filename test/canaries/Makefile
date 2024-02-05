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

.PHONY: macos-canaries
macos-canaries: ansible/dependencies
ifndef ANSIBLE_INVENTORY
	$(error ANSIBLE_INVENTORY is not set)
endif
ifndef MACSTADIUM_USER
	$(error MACSTADIUM_USER \(MacStadium account username for API\) variable must be provided)
endif
ifndef MACSTADIUM_PASS
	$(error MACSTADIUM_PASS \(MacStadium password for API\) variable must be provided)
endif
ifndef MACSTADIUM_SUDO_PASS
	$(error MACSTADIUM_SUDO_PASS \(MacStadium sudo password\) variable must be provided)
endif
	ansible-playbook -f $(ANSIBLE_FORKS) -i $(ANSIBLE_INVENTORY) "$(ANSIBLE_FOLDER)/deploy_macos_canaries.yml" -e "macstadium_user=$(MACSTADIUM_USER) macstadium_pass=$(MACSTADIUM_PASS) macstadium_sudo_pass=$(MACSTADIUM_SUDO_PASS) output_inventory_macos=$(CURDIR)/inventory.macos.ec2 platform=macos"
	sed -i.bak "s/canary:current/canary-macos:v$(VERSION)/g" $(CURDIR)/inventory.macos.ec2
	sed -i.bak "s/canary:previous/canary-macos:v$(PREVIOUS_VERSION)/g" $(CURDIR)/inventory.macos.ec2
	cat $(CURDIR)/inventory.macos.ec2

.PHONY: clean
clean: ansible/clean
