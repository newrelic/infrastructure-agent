ANSIBLE_FOLDER := $(CURDIR)
ANSIBLE_FORKS ?= 20
CROWDSTRIKE_REPO_SSH_KEY ?= $(HOME)/.ssh/crowdstrike_ansible_role_key

.DEFAULT_GOAL := canaries

# Include Ansible dependencies common installation strategy
include ../ansible/common.mk

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

## Conditional Canary Deployment
ifdef IS_A2Q
	    ansible-playbook -f $(ANSIBLE_FORKS) -i $(ANSIBLE_INVENTORY) "$(ANSIBLE_FOLDER)/deploy_a2q_canaries.yml" -e "current_version=$(VERSION) previous_version=$(PREVIOUS_VERSION) nr_license_key_a2q_1=$(NR_LICENSE_KEY_CANARIES_A2Q_1) nr_license_key_a2q_2=$(NR_LICENSE_KEY_CANARIES_A2Q_2) docker_username=$(DOCKER_USERNAME) docker_password=$(DOCKER_PASSWORD) prev=${IS_PREVIOUS}"
else
	    ansible-playbook -f $(ANSIBLE_FORKS) -i $(ANSIBLE_INVENTORY) "$(ANSIBLE_FOLDER)/deploy_canaries.yml" -e "current_version=$(VERSION) previous_version=$(PREVIOUS_VERSION) nr_license_key=$(NR_LICENSE_KEY_CANARIES) docker_username=$(DOCKER_USERNAME) docker_password=$(DOCKER_PASSWORD)"
endif
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
# CrowdStrike Falcon variables
ifndef CROWDSTRIKE_CLIENT_ID
	$(error CROWDSTRIKE_CLIENT_ID \(Crowdstrike client ID\) variable must be provided)
endif
ifndef CROWDSTRIKE_CLIENT_SECRET
	$(error CROWDSTRIKE_CLIENT_SECRET \(Crowdstrike client secret\) variable must be provided)
endif
ifndef CROWDSTRIKE_CUSTOMER_ID
	$(error CROWDSTRIKE_CUSTOMER_ID \(Crowdstrike customer ID\) variable must be provided)
	exit 1
endif
ifndef PREVIOUS_VERSION
	$(error PREVIOUS_VERSION is not set)
endif
	@ansible-playbook -f $(ANSIBLE_FORKS) -i $(ANSIBLE_INVENTORY) "$(ANSIBLE_FOLDER)/inventory_macos_canaries.yml" -e "macstadium_user=$(MACSTADIUM_USER) macstadium_pass=$(MACSTADIUM_PASS) macstadium_sudo_pass=$(MACSTADIUM_SUDO_PASS) output_inventory_macos=$(CURDIR)/inventory.macos.ec2 platform=macos"
	sed -i.bak "s/canary:current/canary-macos:v$(VERSION)/g" $(CURDIR)/inventory.macos.ec2
	sed -i.bak "s/canary:previous/canary-macos:v$(PREVIOUS_VERSION)/g" $(CURDIR)/inventory.macos.ec2
	cat $(CURDIR)/inventory.macos.ec2
	@ansible-playbook -f $(ANSIBLE_FORKS) -i $(CURDIR)/inventory.macos.ec2 "$(ANSIBLE_FOLDER)/deploy_macos_canaries.yml" -e "crowdstrike_client_id=$(CROWDSTRIKE_CLIENT_ID) crowdstrike_client_secret=$(CROWDSTRIKE_CLIENT_SECRET) crowdstrike_customer_id=$(CROWDSTRIKE_CUSTOMER_ID) nr_license_key=$(NR_LICENSE_KEY_CANARIES) current_version=$(VERSION) previous_version=$(PREVIOUS_VERSION)"

.PHONY: clean
clean: ansible/clean
