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

ANSIBLE_DISPLAY_SKIPPED_HOSTS=NO \
  retry \
  ansible-playbook \
  -i $ANSIBLE_INVENTORY \
  -e agent_root_dir=${AGENT_RUN_DIR} \
	-e tests_to_run_regex=${TESTS_TO_RUN_REGEXP} \
	test/harvest/ansible/test.yml
