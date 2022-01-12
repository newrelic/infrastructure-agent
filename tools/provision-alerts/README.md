Create NR1 alerts for canaries.

* Working dir should be infrastructure-agent [root folder](../../)
* AWS credentials/profile should be present

Create alerts:
```shell

NR_API_KEY="" DISPLAY_NAME_CURRENT="" DISPLAY_NAME_PREVIOUS="" TEMPLATE="" make provision-alerts


NR_API_KEY="SOME_NR_USER_API_KEY" \
DISPLAY_NAME_CURRENT="canary:v1.20.7:arm64:redhat-7.6" \
DISPLAY_NAME_PREVIOUS="canary:v1.21.0:arm64:redhat-7.6" \
TEMPLATE="tools/provision-alerts/template/template.yml" \
make provision-alerts

# use optional prefix for policies

NR_API_KEY="SOME_NR_USER_API_KEY" \
DISPLAY_NAME_CURRENT="canary:v1.20.7:arm64:redhat-7.6" \
DISPLAY_NAME_PREVIOUS="canary:v1.21.0:arm64:redhat-7.6" \
TEMPLATE="tools/provision-alerts/template/template.yml" \
PREFIX="[my prefix]" \
make provision-alerts

```

Delete alerts:
```shell

NR_API_KEY="" PREFIX="" make provision-alerts-delete


NR_API_KEY="SOME_NR_USER_API_KEY" \
PREFIX="[my prefix]" \
make provision-alerts-delete

```