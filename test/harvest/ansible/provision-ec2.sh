#!/usr/bin/env bash

ANSIBLE_INVENTORY="$( pwd )/test/harvest/ansible/invntory.local"

printf "\nProvisioning ec2 instances...\n"
if ! ansible-playbook -i "$ANSIBLE_INVENTORY" test/harvest/ansible/provision-ec2.yml; then
  printf "\nProvisioning ec2 instances failed"
  exit 1
fi

