
module "alerts" {
  source = "git::https://github.com/newrelic-experimental/otel-env-provisioner//terraform/nr-alerts?ref=NR-83107_nr_alerts"

  api_key               = var.api_key
  account_id            = var.account_id
  region                = var.region
  instance_name_pattern = var.instance_name_pattern
  policies_prefix       = var.policies_prefix
  conditions            = var.conditions
}
