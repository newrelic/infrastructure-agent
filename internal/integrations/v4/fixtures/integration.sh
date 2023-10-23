#!/usr/bin/env sh

Argument=$(echo "$1" | sed 's|\\|/|g')

echo '{"name":"com.newrelic.test","protocol_version":"1","integration_version":"1.0.0","metrics":[{"event_type":"TestSample","value":"'$Argument'"}]}'
