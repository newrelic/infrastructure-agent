#!/usr/bin/env bash

if [ "$ANSIBLE_INVENTORY" = "" ]; then
  printf "Error: missing required env-var: %s\n" "ANSIBLE_INVENTORY"
  exit 1
fi

AGENT_ROOT_DIR=$( pwd )

printf "\nTesting initial install...\n"
if ! ansible-playbook -i "$ANSIBLE_INVENTORY" -e agent_root_dir="$AGENT_ROOT_DIR" test/harvest/ansible/test.yml; then
  printf "\nRunning the harvest suite failed"
  exit 1
fi

