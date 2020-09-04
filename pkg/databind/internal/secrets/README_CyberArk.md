## Sample configuration for MySql

``` yaml
integration_name: com.newrelic.mysql

variables:
  credentials:
# /opt/CARKaim/sdk/clipasswordsdk GetPassword -p AppDescs.AppID=NewRelic -p Query=Safe=ALL-NERE-WIN-A-NEWRELIC-UP;Folder=Root;Object=ALL-localhost-testuser
#    cyberark-cli:
#      cli: /opt/CARKaim/sdk/clipasswordsdk
#      app-id: NewRelic
#      safe: ALL-NERE-WIN-A-NEWRELIC-UP
#      folder: Root
#      object: ALL-localhost-testuser
    cyberark-api:
      http:
        tls_config:
          insecure_skip_verify: true
        url: https://10.1.0.5/AIMWebService/api/Accounts?AppID=NewRelic&Query=Safe=ALL-NERE-WIN-A-NEWRELIC-UP;Object=ALL-localhost-testuser

instances:
  - name: mysql-server
    command: status
    arguments:
        hostname: localhost
        port: 3306
        username: newrelic
        password: ${credentials.password}
        # New users should leave this property as `true`, to identify the
        # monitored entities as `remote`. Setting this property to `false` (the
        # default value) is deprecated and will be removed soon, disallowing
        # entities that are identified as `local`.
        # Please check the documentation to get more information about local
        # versus remote entities:
        # https://github.com/newrelic/infra-integrations-sdk/blob/master/docs/entity-definition.md
        remote_monitoring: true
    labels:
        env: production
        role: write-replica

```