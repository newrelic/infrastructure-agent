#!/bin/sh
echo '{ "config_protocol_version": "1", "action": "register_config", "config_name": "myconfig-recursive", "config": { "variables": {}, "integrations": [ { "name": "nri-test-recursive-child", "exec": ["echo {}"] } ] } }'
>&2 echo "log stuff"
