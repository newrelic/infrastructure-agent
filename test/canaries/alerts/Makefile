# Include AWS common features
include $(CURDIR)/test/ansible/Ansible.common

# FIXME: Makefile should be able to be run from its relative path
PROVISION_ALERTS_TERRAFORM_WORKSPACE	?= $(CURDIR)/test/canaries/alerts

.DEFAULT_GOAL := provision-alerts-terraform

.PHONY: provision-alerts-terraform-deps
provision-alerts-terraform-deps:
ifndef PREVIOUS_VERSION
	$(error PREVIOUS_VERSION is not set)
endif
ifndef VERSION
	$(error VERSION is not set)
endif
ifndef TF_VAR_tf_state_bucket
	@echo "TF_VAR_tf_state_bucket variable must be provided"
	exit 1
endif
ifndef TF_VAR_tf_state_region
	@echo "TF_VAR_tf_state_region variable must be provided"
	exit 1
endif
ifndef TF_VAR_tf_state_key
	@echo "TF_VAR_tf_state_key variable must be provided"
	exit 1
endif
ifndef TF_VAR_api_key
	@echo "TF_VAR_api_key variable must be provided"
	exit 1
endif
ifndef TF_VAR_account_id
	@echo "TF_VAR_account_id variable must be provided"
	exit 1
endif
ifndef TF_VAR_region
	@echo "TF_VAR_region variable must be provided"
	exit 1
endif
ifeq ("$(wildcard $(PROVISION_ALERTS_TERRAFORM_WORKSPACE)/vars.auto.tfvars)","")
	@cp $(PROVISION_ALERTS_TERRAFORM_WORKSPACE)/vars.auto.tfvars.dist $(PROVISION_ALERTS_TERRAFORM_WORKSPACE)/vars.auto.tfvars
endif
	sed -i.bak "s/vVERSION:v$(VERSION)/g" $(PROVISION_ALERTS_TERRAFORM_WORKSPACE)/vars.auto.tfvars
	sed -i.bak "s/vPREVIOUS_VERSION:v$(PREVIOUS_VERSION)/g" $(PROVISION_ALERTS_TERRAFORM_WORKSPACE)/vars.auto.tfvars
	@cd $(PROVISION_ALERTS_TERRAFORM_WORKSPACE) \
    && terraform get -update \
    && terraform init -reconfigure \
       -backend-config "bucket=$(TF_VAR_tf_state_bucket)" \
       -backend-config "region=$(TF_VAR_tf_state_region)" \
       -backend-config "key=$(TF_VAR_tf_state_key)"

.PHONY: provision-alerts-terraform
provision-alerts-terraform: validate-aws-credentials provision-alerts-terraform-deps
	@cd $(PROVISION_ALERTS_TERRAFORM_WORKSPACE) \
	&& terraform apply -auto-approve

.PHONY: clean
clean: validate-aws-credentials provision-alerts-terraform-deps
	@cd $(PROVISION_ALERTS_TERRAFORM_WORKSPACE) \
	&& terraform destroy -auto-approve
