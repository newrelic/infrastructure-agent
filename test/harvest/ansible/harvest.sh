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

ANSIBLE_DISPLAY_SKIPPED_HOSTS=NO \
  retry \
  ansible-playbook \
  -i $ANSIBLE_INVENTORY \
  -e agent_root_dir=${AGENT_RUN_DIR} \
	-e tests_to_run_regex=${TESTS_TO_RUN_REGEXP} \
	test/harvest/ansible/test.yml
