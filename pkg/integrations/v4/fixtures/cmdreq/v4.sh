#!/bin/sh
echo '{"name":"com.newrelic.shelltest","protocol_version":"3","integration_version":"0.0.0","data":[{"entity":{"name":"some-entity","type":"shell-test","id_attributes":[]},"metrics":[{"event_type":"ShellTestSample","some-metric":1}],"inventory":{"foo":{"name":"bar"}},"events":[]}]}'
>&2 echo "log stuff"
