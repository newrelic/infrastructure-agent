#!/bin/bash

if [ "$ANSIBLE_INVENTORY" = "" ]; then
  printf "Error: missing required env-var: %s\n" "ANSIBLE_INVENTORY"
  exit 1
fi

printf "\nProvisioning boxes lacking python...\n"
ansible rhel_8,centos_8 --become -m raw -a "yum install -y python3"


printf "\nAccepting boxes SSH keys...\n"
ansible-playbook -i "$ANSIBLE_INVENTORY" ansible/initial_setup.yml
