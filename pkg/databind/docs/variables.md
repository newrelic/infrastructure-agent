# Secret Management

With secrets management, you can configure the agent and on-host integrations to use sensitive data (such as passwords)
without having to write them as plain text into the configuration files. Currently, Hashicorp Vault, AWS KMS, CyberArk,
and New Relic CLI obfuscation are supported.

You can use the integration configuration option `variables` to fetch secret data. It accepts many entries. For each entry,
only one secret will be retrieved, even if this secret is structured with many fields.
If the secret is not found, the discovery process fails and return an error.

```yaml
variables:
  creds:
    vault:
      http:
        url: http://my.vault.host/v1/newengine/data/secret
        headers:
          X-Vault-Token: my-vault-token
          
integrations:
  - name: nri-nginx
    env:
      METRICS: "true"
      STATUS_URL: http://${creds.user}:${creds.password}@example.com/status
      STATUS_MODULE: discover
      REMOTE_MONITORING: true
    interval: 30s
    labels:
      env: production
      role: load_balancer
```

For more information check our [public documentation](https://docs.newrelic.com/docs/infrastructure/host-integrations/installation/secrets-management/).
