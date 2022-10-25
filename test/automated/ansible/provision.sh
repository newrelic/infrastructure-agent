#!/usr/bin/env bash

function retry () {
  retries=5
  until [ $retries -eq 0 ]
  do
    "$@"
    status="$?"
    if [ "$status" -eq 0 ];then
      break
    fi
    ((retries--))
  done

  return "$status"
}

ANSIBLE_STDOUT_CALLBACK=selective \
  ansible-playbook \
  -i test/automated/ansible/inventory.local \
  -e provision_host_prefix=$PROVISION_HOST_PREFIX \
  -e output_inventory_ext=$ANSIBLE_INVENTORY \
  -e ansible_password_windows=$ANSIBLE_PASSWORD_WINDOWS \
  -e platform=$PLATFORM \
  -f $ANSIBLE_FORKS \
  test/automated/ansible/provision.yml

ANSIBLE_DISPLAY_SKIPPED_HOSTS=NO retry ansible-playbook -f $ANSIBLE_FORKS -i $ANSIBLE_INVENTORY test/automated/ansible/install-requirements.yml

if [[ "$PLATFORM" == "macos" || "$PLATFORM" == "all" ]];then
  retry ansible-playbook -e macstadium_user=$MACSTADIUM_USER -e macstadium_sudo_pass=$MACSTADIUM_SUDO_PASS -e macstadium_pass=$MACSTADIUM_PASS test/automated/ansible/macos-canaries.yml
fi
