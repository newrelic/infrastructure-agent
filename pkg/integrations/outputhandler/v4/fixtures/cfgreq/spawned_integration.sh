#!/bin/sh
case $STDOUT_TYPE in
# Outputs a config request 
cfgreq)
    echo '{ "config_protocol_version": "1", "action": "register_config", "config_name": "myconfig-recursive", "config": { "variables": {}, "integrations": [ { "name": "nri-test-recursive-child", "exec": ["echo {}"] } ] } }'
    ;;
# Outputs a integration payload v3
v3)
    echo '{"name":"com.newrelic.shelltest","protocol_version":"3","integration_version":"0.0.0","data":[{"entity":{"name":"some-entity","type":"shell-test","id_attributes":[]},"metrics":[{"event_type":"ShellTestSample","some-metric":1}],"inventory":{"foo":{"name":"bar"}},"events":[]}]}'
    ;;
# Outputs a integration payload v4
v4)
    echo '{ "protocol_version": "4", "integration": { "name": "Foo", "version": "1.0.0" }, "data": [ { "common": { "timestamp": 1586357933, "interval.ms": 10000, "attributes": { "host.name": "host-foo", "host.user": "foo-man-choo" } }, "metrics": [ { "name": "a.gauge", "type": "gauge", "value": 13, "attributes": { "key1": "val1" } } ], "entity": { "name": "a.entity.name", "type": "ASample", "displayName": "A display name", "tags": { "env": "testing" } }, "inventory": { "foo": { "value": "bar" } }, "events": [] } ] }'
    ;;
esac
>&2 echo "log stuff"
