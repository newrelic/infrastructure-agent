module "alerts" {
  source = "git::https://github.com/newrelic-experimental/env-provisioner//terraform/nr-alerts"

  api_key               = var.api_key
  account_id            = var.account_id
  region                = var.region
  policies_prefix       = var.policies_prefix
  conditions            = var.conditions
  display_names         = flatten([
        var.windows_display_names,
        var.linux_display_names,
        var.macos_display_names
  ])
}
