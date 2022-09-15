#!/bin/bash


CLIARGS="{"
for arg in "$@"
do
	keyVal=(${arg//=/ })
	CLIARGS+="\"${keyVal[0]}\":\"${keyVal[1]}\","
done

# remove last comma
CLIARGS=${CLIARGS%?}
CLIARGS+="}"

INVENTORY_JSON='{"name":"testing_integration","protocol_version":"2","integration_version":"1.2.3","integration_status":"","data":[{"entity":{"name":"localtest","type":"test","id_attributes":[{"Key":"idkey","Value":"idval"}],"displayName":"","metadata":null},"metrics":[],"inventory":{"cliargs":%s},"events":[],"add_hostname":false,"cluster":"local-test","service":"test-service"}]}\n'

printf "$INVENTORY_JSON" "$CLIARGS" 
