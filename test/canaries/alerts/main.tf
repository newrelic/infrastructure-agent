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

resource "newrelic_notification_channel" "sre_agent_channel" {
  name           = "Canary Alerts SRE Agent Channel"
  account_id     = var.account_id
  destination_id = "0b7bf32c-f336-442e-8be4-42e5f3eda718"
  product        = "IINT"
  type           = "WEBHOOK"

  property {
    key   = "headers"
    value = "{\"x-respond-async\":\"true\",\"x-Account-Id\":\"{{nrAccountId}}\"}"
  }

  property {
    key   = "payload"
    value = <<-EOT
    {"messages":[{"role":"user","content":"I was paged for an alert issue '{{ issueId }}' for entity '{{ entitiesData.names.[0] }}'. This is a canary alert comparing infrastructure agent versions. What seems to be the cause of this alert? Provide detailed analysis of the historical trends on the alert signal.","context":[{"state":"{{ state }}","accountId":"{{ nrAccountId }}","status":"{{ status }}","triggerEvent":"{{ triggerEvent }}","activatedAt":"{{ activatedAt }}","acknowledgedAt":"{{ acknowledgedAt }}","acknowledgedByChannel":"{{#if acknowledgedByChannel}}{{ acknowledgedByChannel }}{{else}}N/A{{/if}}","closedByChannel":"{{#if closedByChannel}}{{ closedByChannel }}{{else}}N/A{{/if}}","closedAt":"{{ closedAt }}","createdAt":"{{ createdAt }}","unAcknowledgedAt":"{{#if unAcknowledgedAt}}{{ unAcknowledgedAt }}{{else}}N/A{{/if}}","updatedAt":"{{ updatedAt }}","issueId":"{{ issueId }}","mutingState":"{{ mutingState }}","isCorrelated":"{{ isCorrelated }}","correlatedBy":"{{#if correlatedBy}}{{ correlatedBy }}{{else}}N/A{{/if}}","priority":"{{ priority }}"}]}]}
    EOT
  }
}

resource "newrelic_workflow" "canary_alerts_workflow" {
  name                  = "Infra Agent Canary Alerts - SRE Agent"
  account_id            = var.account_id
  muting_rules_handling = "DONT_NOTIFY_FULLY_MUTED_ISSUES"
  enabled               = true

  issues_filter {
    name = "canary-policy-filter"
    type = "FILTER"

    predicate {
      attribute = "labels.policyName"
      operator  = "CONTAINS"
      values    = [var.policies_prefix]
    }
  }

  destination {
    channel_id              = newrelic_notification_channel.sre_agent_channel.id
    notification_triggers   = ["ACTIVATED"]
    update_original_message = true
  }

  depends_on = [module.alerts]
}
