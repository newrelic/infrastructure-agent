#!/usr/bin/env bash

ANSIBLE_INVENTORY="$( pwd )/test/harvest/ansible/inventory.local"

printf "\nTerminating ec2 instances...\n"
if ! ansible-playbook -i "$ANSIBLE_INVENTORY" test/harvest/ansible/terminate-ec2.yml; then
  printf "\nTerminating ec2 instances failed"
  exit 1
fi

