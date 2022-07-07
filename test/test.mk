PROVISION_HOST_PREFIX := $(shell whoami)-$(shell hostname)
AWS_ACCOUNT_ID = "018789649883"# CAOS

ANSIBLE_INVENTORY ?= $(CURDIR)/test/automated/ansible/inventory.ec2

.PHONY: test/automated/provision
test/automated/provision: validate-aws-credentials
ifndef ANSIBLE_PASSWORD_WINDOWS
	@echo "ANSIBLE_PASSWORD_WINDOWS variable must be provided for test/automated/provision"
	exit 1
endif
	PROVISION_HOST_PREFIX=$(PROVISION_HOST_PREFIX) $(CURDIR)/test/automated/ansible/provision.sh

.PHONY: test/automated/termination
test/automated/termination: validate-aws-credentials
	ansible-playbook -i $(ANSIBLE_INVENTORY) $(CURDIR)/test/automated/ansible/termination.yml

# Allow running specific harvest tests based on regex (default to .*)
TESTS_TO_RUN_REGEXP ?= ".*"
.PHONY: test/automated/harvest
test/automated/harvest:
	AGENT_RUN_DIR=$(CURDIR) $(CURDIR)/test/harvest/ansible/harvest.sh

.PHONY: test/automated/packaging
test/automated/packaging:
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
	@ANSIBLE_DISPLAY_SKIPPED_HOSTS=NO ANSIBLE_DISPLAY_OK_HOSTS=NO ansible-playbook -i $(ANSIBLE_INVENTORY) -e nr_license_key=$(NR_LICENSE_KEY) -e nr_api_key=$(NEW_RELIC_API_KEY) -e nr_account_id=$(NEW_RELIC_ACCOUNT_ID) $(CURDIR)/test/packaging/ansible/test.yml


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
	make test/automated/packaging-docker
	#packaging tests will terminate provisioned instances to test HNR alerts so should be the last one
	make test/automated/packaging

.PHONY: test/runner/provision
test/runner/provision: GIT_REF ?= "HEAD"
test/runner/provision:
	@ANSIBLE_DISPLAY_SKIPPED_HOSTS=NO ANSIBLE_DISPLAY_OK_HOSTS=NO ansible-playbook -i $(CURDIR)/test/automated/ansible/inventory.runner.ec2 -e git_ref=$(GIT_REF) $(CURDIR)/test/automated/ansible/provision-runner.yml

.PHONY: test/runner/packaging
test/runner/packaging: validate-aws-credentials
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
ifndef RUNNER_IP
	@echo "RUNNER_IP variable must be provided for test/runner/packaging"
	exit 1
endif
ifndef SSH_KEY
	@echo "SSH_KEY variable must be provided for test/runner/packaging"
	exit 1
endif

	@echo '#!/usr/bin/env bash' > '/tmp/runner_scr.sh'
	@echo 'LOG_FILE="/var/log/runner/$$(date '+%Y%m%d_%H%M').log"' >> /tmp/runner_scr.sh
	@echo 'cd /home/ubuntu/dev/newrelic/infrastructure-agent' >> /tmp/runner_scr.sh
	@echo 'date > $$LOG_FILE' >> /tmp/runner_scr.sh
	@echo 'make test/automated-run 2>&1 >> $$LOG_FILE' >> /tmp/runner_scr.sh
	@echo 'echo "" >> $$LOG_FILE' >> /tmp/runner_scr.sh
	@echo 'date >> $$LOG_FILE' >> /tmp/runner_scr.sh
	@echo 'mail -s "[RUNNER] Packaging tests results" caos-dev@newrelic.com -A $$LOG_FILE < $$LOG_FILE' >> /tmp/runner_scr.sh
	@chmod +x /tmp/runner_scr.sh

	make test/runner/provision

	scp -i $(SSH_KEY) /tmp/runner_scr.sh ubuntu@$(RUNNER_IP):/home/ubuntu/runner_scr.sh
	ssh -i $(SSH_KEY) -f ubuntu@$(RUNNER_IP) "NR_LICENSE_KEY=$(NR_LICENSE_KEY) NEW_RELIC_API_KEY=$(NEW_RELIC_API_KEY) NEW_RELIC_ACCOUNT_ID=$(NEW_RELIC_ACCOUNT_ID) nohup /home/ubuntu/runner_scr.sh > /dev/null 2>&1 &"
