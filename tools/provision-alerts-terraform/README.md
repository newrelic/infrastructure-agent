## Provision alerts using Terraform

### Provision Alerts using make target

```
make provision-alerts-terraform
TF_VAR_api_key=************ \
TF_VAR_account_id=000000 \
TF_VAR_region=US \
TF_VAR_instance_name_pattern=canary:v1.32.7:* \
TF_VAR_instance_policies_prefix="[pre-release] Canaries metric comparator" \
TF_VAR_tf_state_bucket=some_bucket_for_state \
TF_VAR_tf_state_region=aws_region_for_state \
TF_VAR_tf_state_key=key_for_state
```


### Delete Alerts using make target

```
make provision-alerts-terraform-delete
TF_VAR_api_key=************ \
TF_VAR_account_id=000000 \
TF_VAR_region=US \
TF_VAR_instance_name_pattern=canary:v1.32.7:* \
TF_VAR_instance_policies_prefix="[pre-release] Canaries metric comparator" \
TF_VAR_tf_state_bucket=some_bucket_for_state \
TF_VAR_tf_state_region=aws_region_for_state \
TF_VAR_tf_state_key=key_for_state
```

### Execute manually
Copy [vars.auto.tfvars.dist](vars.auto.tfvars.dist) to [vars.auto.tfvars](vars.auto.tfvars) and 
fill the current variables:
```yaml
api_key               = "" # NR User Api Key
account_id            = 0 # NR Account ID
region                = "" # US | EU | Staging
instance_name_pattern = "canary:v0.0.0:*" # ec2 instances name pattern
policies_prefix       = "[pre-release] Canaries metric comparator"
```

Execute:
```shell
terraform get 
terraform init \
    -backend-config "bucket=some_bucket" \
    -backend-config "region=aws_region" \
    -backend-config "key=key_for_state"
terraform apply
```

Delete alerts manually:
```shell
terraform get 
terraform init \
    -backend-config "bucket=some_bucket" \
    -backend-config "region=aws_region" \
    -backend-config "key=key_for_state"
terraform destroy
```