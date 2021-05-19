#!/bin/bash

if [ "$ANSIBLE_INVENTORY" = "" ]; then
  printf "Error: missing required env-var: %s\n" "ANSIBLE_INVENTORY"
  exit 1
fi

if [ "$NR_LICENSE_KEY" = "" ]; then
  printf "Error: missing required env-var: %s\n" "NR_LICENSE_KEY"
  exit 1
fi

if [ "$AGENT_VERSION" = "" ]; then
  printf "Error: missing required env-var: %s\n" "AGENT_VERSION"
  exit 1
fi

printf "\nTesting initial install...\n"
if ! NR_LICENSE_KEY="$NR_LICENSE_KEY" ansible-playbook -i "$ANSIBLE_INVENTORY" test/packaging/ansible/test.yml; then
  printf "\nRunning the test suite failed"
  exit 1
fi

printf "\nVerify integrations in docker are run in expected arch...\n"

DOCKER_IMAGE="newrelic/infrastructure:$AGENT_VERSION-rc"
if ! docker run --rm --entrypoint /var/db/newrelic-infra/newrelic-integrations/bin/nri-prometheus "$DOCKER_IMAGE" "--help"; then
  printf "\nFailed running integration nri-prometheus"
  exit 1
fi

if ! docker run --rm --entrypoint /var/db/newrelic-infra/newrelic-integrations/bin/nri-flex "$DOCKER_IMAGE" "--help"; then
  printf "\nFailed running integration nri-flex"
  exit 1
fi

if ! docker run --rm --entrypoint /var/db/newrelic-infra/newrelic-integrations/bin/nri-docker "$DOCKER_IMAGE" "-show_version"; then
  printf "\nFailed running integration nri-docker"
  exit 1
fi