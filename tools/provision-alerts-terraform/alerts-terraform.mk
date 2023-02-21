PROVISION_ALERTS_TERRAFORM_WORKSPACE	?= $(CURDIR)/tools/provision-alerts-terraform

.PHONY: provision-alerts-terraform-deps
provision-alerts-terraform-deps:
ifndef TF_VAR_api_key
	@echo "TF_VAR_api_key variable must be provided"
	exit 1
endif
ifndef TF_VAR_account_id
	@echo "TF_VAR_api_key variable must be provided"
	exit 1
endif
ifndef TF_VAR_region
	@echo "TF_VAR_region variable must be provided"
	exit 1
endif
ifndef TF_VAR_instance_name_pattern
	@echo "TF_VAR_instance_name_pattern variable must be provided"
	exit 1
endif
ifndef TF_VAR_instance_policies_prefix
	@echo "TF_VAR_instance_policies_prefix variable must be provided"
	exit 1
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
	@cd $(PROVISION_ALERTS_TERRAFORM_WORKSPACE) \
    && terraform get \
    && terraform init \
       -backend-config "bucket=$(TF_VAR_tf_state_bucket)" \
       -backend-config "region=$(TF_VAR_tf_state_region)" \
       -backend-config "key=$(TF_VAR_tf_state_key)"

.PHONY: provision-alerts-terraform
provision-alerts-terraform: validate-aws-credentials provision-alerts-terraform-deps
	@cd $(PROVISION_ALERTS_TERRAFORM_WORKSPACE) \
	&& terraform apply -auto-approve

.PHONY: provision-alerts-terraform-delete
provision-alerts-terraform-delete: validate-aws-credentials provision-alerts-terraform-deps
	@cd $(PROVISION_ALERTS_TERRAFORM_WORKSPACE) \
	&& terraform destroy -auto-approve