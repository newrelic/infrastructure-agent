# Include AWS common features
include ../../ansible/common.mk
include ../../common.mk

PROVISION_ALERTS_TERRAFORM_WORKSPACE	?= $(CURDIR)

.DEFAULT_GOAL := provision-alerts-terraform

.PHONY: provision-alerts-terraform-deps
provision-alerts-terraform-deps:
ifndef TERRAFORM_STATE_KEY
	$(error TERRAFORM_STATE_KEY is undefined)
endif
ifndef PREVIOUS_VERSION
	$(error PREVIOUS_VERSION is not set)
endif
ifndef VERSION
	$(error VERSION is not set)
endif
ifndef MACSTADIUM_USER
	$(error MACSTADIUM_USER is not set)
endif
ifndef MACSTADIUM_PASSWORD
	$(error MACSTADIUM_PASSWORD is not set)
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
	sed "s/TERRAFORM_STATE_KEY/${TERRAFORM_STATE_KEY}/g" "$(PROVISION_ALERTS_TERRAFORM_WORKSPACE)/state.tf.dist" > "$(PROVISION_ALERTS_TERRAFORM_WORKSPACE)/state.tf"
	@MACSTADIUM_USER="$(MACSTADIUM_USER)" MACSTADIUM_PASSWORD="$(MACSTADIUM_PASSWORD)" $(PROVISION_ALERTS_TERRAFORM_WORKSPACE)/fetch_instances.sh $(VERSION) $(PREVIOUS_VERSION) >> $(PROVISION_ALERTS_TERRAFORM_WORKSPACE)/vars.auto.tfvars
	@cd $(PROVISION_ALERTS_TERRAFORM_WORKSPACE) \
    && terraform get -update \
    && terraform init -reconfigure

.PHONY: provision-alerts-terraform
provision-alerts-terraform: validate-aws-credentials provision-alerts-terraform-deps
	@cd $(PROVISION_ALERTS_TERRAFORM_WORKSPACE) \
	&& terraform apply -auto-approve

.PHONY: alerts-clean
alerts-clean: validate-aws-credentials provision-alerts-terraform-deps
	@cd $(PROVISION_ALERTS_TERRAFORM_WORKSPACE) \
	&& terraform destroy -auto-approve
