#!/usr/bin/env bash

if [ "$ANSIBLE_INVENTORY" = "" ]; then
  printf "Error: missing required env-var: %s\n" "ANSIBLE_INVENTORY"
  exit 1
fi

if [ "$AGENT_VERSION" = "" ]; then
  printf "Error: missing required env-var: %s\n" "AGENT_VERSION"
  exit 1
fi

printf "\nTesting initial install...\n"
if ! ansible-playbook -i "$ANSIBLE_INVENTORY" -e agent_version="$AGENT_VERSION" test/harvest/ansible/test.yml; then
  printf "\nRunning the harvest suite failed"
  exit 1
fi

