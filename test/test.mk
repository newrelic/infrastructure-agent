PROVISION_HOST_PREFIX := $(shell whoami)-$(shell hostname)

.PHONY: test/automated/provision
test/automated/provision: validate-aws-credentials
	ansible-playbook -i $(CURDIR)/test/automated/ansible/inventory.local -e provision_host_prefix=$(PROVISION_HOST_PREFIX) $(CURDIR)/test/automated/ansible/provision.yml
	ANSIBLE_DISPLAY_SKIPPED_HOSTS=NO ansible-playbook -i $(CURDIR)/test/automated/ansible/inventory.ec2 $(CURDIR)/test/automated/ansible/install-requirements.yml

.PHONY: test/automated/termination
test/automated/termination: validate-aws-credentials
	ansible-playbook -i $(CURDIR)/test/automated/ansible/inventory.ec2 $(CURDIR)/test/automated/ansible/termination.yml

# Allow running specific harvest tests based on regex (default to .*)
TESTS_TO_RUN_REGEXP ?= ".*"
.PHONY: test/automated/harvest
test/automated/harvest:
	ANSIBLE_DISPLAY_SKIPPED_HOSTS=NO ansible-playbook -i $(CURDIR)/test/automated/ansible/inventory.ec2 \
					-e agent_root_dir=$(CURDIR) \
					-e tests_to_run_regex=$(TESTS_TO_RUN_REGEXP) \
					$(CURDIR)/test/harvest/ansible/test.yml

.PHONY: test/automated/packaging
test/automated/packaging:
ifndef NR_LICENSE_KEY
	@echo "NR_LICENSE_KEY variable must be provided for test/automated/packaging"
	exit 1
else
	@ANSIBLE_DISPLAY_SKIPPED_HOSTS=NO ANSIBLE_DISPLAY_OK_HOSTS=NO ansible-playbook -i $(CURDIR)/test/automated/ansible/inventory.ec2 -e nr_license_key=$(NR_LICENSE_KEY) $(CURDIR)/test/packaging/ansible/test.yml
endif

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
ifndef AWS_PROFILE
	@echo "AWS_PROFILE variable must be provided"
	exit 1
endif
ifndef AWS_REGION
	@echo "AWS_REGION variable must be provided"
	exit 1
endif

.PHONY: test/automated
test/automated:
	$(MAKE) test/automated-run || $(MAKE) test/automated/nuke

.PHONY: test/automated/nuke
test/automated/nuke: validate-aws-credentials
ifeq ($(PROVISION_HOST_PREFIX),)
	@echo "PROVISION_HOST_PREFIX is empty"
	exit 1
endif
	IIDS="$(shell AWS_PAGER="" aws ec2 describe-instances --output text --region us-east-2 \
		--filters 'Name=tag:Name,Values="$(PROVISION_HOST_PREFIX):*"' 'Name=instance-state-name,Values=running' \
		--query 'Reservations[*].Instances[*].[InstanceId]' )"; \
	@echo "Terminating instances: $$IIDS ..."; \
	AWS_PAGER="" aws ec2 terminate-instances --instance-ids $$IIDS;

.PHONY: test/automated-run
test/automated-run:
	make test/automated/provision
	make test/automated/harvest
	make test/automated/packaging
	make test/automated/packaging-docker
	make test/automated/termination
