
module "alerts" {
  source = "/Users/rcoll/Documents/github/newrelic-experimental/env-provisioner/terraform/nr-alerts"

  api_key               = var.api_key
  account_id            = var.account_id
  region                = var.region
  policies_prefix       = var.policies_prefix
  conditions            = var.conditions
  display_names         = flatten([
        var.windows_display_names,
        var.sles_display_names,
        var.redhat_display_names,
        var.debian_display_names,
        var.amazonlinux_display_names,
        var.centos_display_names,
        var.ubuntu_display_names
  ])
}
