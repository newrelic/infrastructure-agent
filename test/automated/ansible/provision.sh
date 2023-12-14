#!/usr/bin/env bash

function retry () {
  retries=5
  until [ "$retries" -eq 0 ]
  do
    "$@"
    status="$?"
    if [ "$status" -eq 0 ];then
      break
    fi
    ((retries--))

    if [ "$retries" -gt 0 ]; then
      echo '[ALLOW_MSG]: execution failed, retrying...'
    fi
  done

  return "$status"
}

if [[ "$PLATFORM" != "macos" ]];then
ANSIBLE_STDOUT_CALLBACK=selective \
  ansible-playbook \
  -i test/automated/ansible/inventory.local \
  -e provision_host_prefix=$PROVISION_HOST_PREFIX \
  -e output_inventory_ext=$ANSIBLE_INVENTORY \
  -e ansible_password_windows=$ANSIBLE_PASSWORD_WINDOWS \
  -e platform=$PLATFORM \
  -f $ANSIBLE_FORKS \
  test/automated/ansible/provision.yml
fi

ANSIBLE_DISPLAY_SKIPPED_HOSTS=NO \
  retry ansible-playbook \
  -f "$ANSIBLE_FORKS" \
  -i "$ANSIBLE_INVENTORY" \
  -e "crowdstrike_client_id=$CROWDSTRIKE_CLIENT_ID" \
  -e "crowdstrike_client_secret=$CROWDSTRIKE_CLIENT_SECRET" \
  -e "crowdstrike_customer_id=$CROWDSTRIKE_CUSTOMER_ID" \
  test/automated/ansible/install-requirements.yml
if [ $? -ne 0 ];then
  echo "install-requirements.yml failed"
  exit 1
fi

if [[ "$PLATFORM" == "macos" || "$PLATFORM" == "all" ]];then
  retry ansible-playbook \
  -e macstadium_user=$MACSTADIUM_USER \
  -e macstadium_sudo_pass=$MACSTADIUM_SUDO_PASS \
  -e macstadium_pass=$MACSTADIUM_PASS \
   -e output_inventory_macos=$ANSIBLE_INVENTORY \
  test/automated/ansible/macos-canaries.yml
fi
