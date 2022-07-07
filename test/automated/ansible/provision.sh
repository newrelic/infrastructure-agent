#!/usr/bin/env bash

function retry () {
  retries=5
  until [ $retries -eq 0 ]
  do
    "$@"
    if [ $? -eq 0 ];then
      break
    fi
    ((retries--))
  done
}

ANSIBLE_STDOUT_CALLBACK=selective \
  ansible-playbook \
  -i test/automated/ansible/inventory.local \
  -e provision_host_prefix=$PROVISION_HOST_PREFIX \
  -e output_inventory_ext=$ANSIBLE_INVENTORY \
  -e ansible_password_windows=$ANSIBLE_PASSWORD_WINDOWS \
  test/automated/ansible/provision.yml

ANSIBLE_DISPLAY_SKIPPED_HOSTS=NO retry ansible-playbook -i $ANSIBLE_INVENTORY test/automated/ansible/install-requirements.yml

retry ansible-playbook -e macstadium_user=$MACSTADIUM_USER -e macstadium_pass=$MACSTADIUM_PASS test/automated/ansible/macos-canaries.yml
