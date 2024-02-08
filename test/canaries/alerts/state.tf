#########################################
# State S3 Backend
#
# Variables cannot be used in the state backend
# solution c&p from:
# https://github.com/hashicorp/terraform/issues/13022#issuecomment-1252347403
#
# to be used with:
#
# terraform init \
#       -backend-config "bucket=$TF_VAR_tf_state_bucket" \
#       -backend-config "region=$TF_VAR_region" \
#       -backend-config "key=$TF_VAR_application/$TF_VAR_environment"
#########################################
terraform {
  backend "s3" {}
}
