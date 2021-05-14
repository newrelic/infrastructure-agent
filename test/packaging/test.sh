#!/bin/bash

if [ "$ANSIBLE_INVENTORY" = "" ]; then
  printf "Error: missing required env-var: %s\n" "ANSIBLE_INVENTORY"
  exit 1
fi

if [ "$NR_LICENSE_KEY" = "" ]; then
  printf "Error: missing required env-var: %s\n" "NR_LICENSE_KEY"
  exit 1
fi

printf "\nTesting initial install...\n"
if ! NR_LICENSE_KEY="$NR_LICENSE_KEY" ansible-playbook -i "$ANSIBLE_INVENTORY" test/packaging/ansible/test.yml; then
  printf "\nRunning the test suite failed"
  exit 1
fi

printf "\nVerify integrations in docker are run in expected arch...\n"

if ! docker run --rm --entrypoint /var/db/newrelic-infra/newrelic-integrations/bin/nri-prometheus newrelic/infrastructure "--help"; then
  printf "\nFailed running integration nri-prometheus"
  exit 1
fi

if ! docker run --rm --entrypoint /var/db/newrelic-infra/newrelic-integrations/bin/nri-flex newrelic/infrastructure "--help"; then
  printf "\nFailed running integration nri-flex"
  exit 1
fi

if ! docker run --rm --entrypoint /var/db/newrelic-infra/newrelic-integrations/bin/nri-docker newrelic/infrastructure "-show_version"; then
  printf "\nFailed running integration nri-docker"
  exit 1
fi