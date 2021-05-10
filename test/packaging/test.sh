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
NR_LICENSE_KEY="$NR_LICENSE_KEY" ansible-playbook -i "$ANSIBLE_INVENTORY" test/packaging/ansible/test.yml
