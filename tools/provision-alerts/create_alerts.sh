#!/bin/bash

INVENTORY=$1
API_KEY=$2
TAG=$3

export TEMPLATE="tools/provision-alerts/template/template.yml"
export PREFIX="[pre-release]"
export PREVIOUS=$(tools/spin-ec2/bin/spin-ec2 canaries previous_canary_version -t $TAG)
echo "previous: $PREVIOUS"
make provision-alerts-build
for CURRENT in $( cat $INVENTORY | grep -v $PREVIOUS | grep linux | grep ansible_host | awk '{print $1}' );do
  export DISPLAY_NAME_CURRENT="${CURRENT}-docker-current"
  export DISPLAY_NAME_PREVIOUS="${CURRENT}-docker-previous"
  echo "${DISPLAY_NAME_CURRENT}:${DISPLAY_NAME_PREVIOUS}"
  NR_API_KEY=$API_KEY make provision-alerts
done
for CURRENT in $( cat $INVENTORY | grep -v $PREVIOUS | grep -v linux | grep ansible_host | awk '{print $1}' );do
  export DISPLAY_NAME_CURRENT=$CURRENT
  export DISPLAY_NAME_PREVIOUS=$( echo $DISPLAY_NAME_CURRENT | sed 's/canary:[^:]*:/canary:v'$PREVIOUS':/g' )
  echo "${DISPLAY_NAME_CURRENT}:${DISPLAY_NAME_PREVIOUS}"
  NR_API_KEY=$API_KEY make provision-alerts
done
