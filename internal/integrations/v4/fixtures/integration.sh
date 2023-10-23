#!/usr/bin/env sh

# Avoid later deserialization problems when receiving a windows path
Argument=$(echo "$1" | sed 's|\\|/|g')

echo '{"name":"com.newrelic.test","protocol_version":"1","integration_version":"1.0.0","metrics":[{"event_type":"TestSample","value":"'$Argument'"}]}'
