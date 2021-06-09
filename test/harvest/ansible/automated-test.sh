#!/usr/bin/env bash

# Ensure aws conf exists
if [ "$AWS_PROFILE" = "" ]; then
  printf "Error: missing required env-var: %s\n" "AWS_PROFILE"
  exit 1
fi
if [ "$AWS_REGION" = "" ]; then
  printf "Error: missing required env-var: %s\n" "AWS_REGION"
  exit 1
fi

AGENT_ROOT_DIR=$(pwd)
# Provision/terminate ec2 instances
ANSIBLE_LOCAL_INVENTORY="$AGENT_ROOT_DIR/test/harvest/ansible/inventory.local"
# Build and run harvest tests in provisioned instances
HARVEST_TESTS_ANSIBLE_INVENTORY="$AGENT_ROOT_DIR/test/harvest/ansible/inventory.ec2"
# Allow not provisioning/terminating instances for testing purposes
TERMINATE=${TERMINATE:-1}
PROVISION=${PROVISION:-1}
# Allow running specific harvest tests based on regex (default to .*)
TESTS_TO_RUN_REGEXP=${TESTS_TO_RUN_REGEXP:-.*}

if [ "$TERMINATE" -eq 1 ]; then
  printf "\nEnsure there are no running instances...\n"
  if ! ansible-playbook -i "$ANSIBLE_LOCAL_INVENTORY" "$AGENT_ROOT_DIR/test/harvest/ansible/terminate-ec2.yml"; then
    printf "\nEnsuring no ec2 instance is running failed"
    exit 1
  fi
fi

if [ "$PROVISION" -eq 1 ]; then
  printf "\nProvisioning ec2 instances...\n"
  if ! ansible-playbook -i "$ANSIBLE_LOCAL_INVENTORY" "$AGENT_ROOT_DIR/test/harvest/ansible/provision-ec2.yml"; then
    printf "\nProvisioning ec2 instances failed"
    exit 1
  fi
fi

# Ensure GOBIN is empty to allow cross-compilation
export GOBIN=""
# Capture exit code of the tests to be returned after terminating instances
EXIT_CODE=0
printf "\nTesting initial install...\n"
ansible-playbook -i "$HARVEST_TESTS_ANSIBLE_INVENTORY" \
  -e agent_root_dir="$AGENT_ROOT_DIR" \
  -e tests_to_run_regex="$TESTS_TO_RUN_REGEXP" \
  "$AGENT_ROOT_DIR/test/harvest/ansible/test.yml"

EXIT_CODE=$?
if [ $EXIT_CODE -ne 0 ]; then
  printf "\nRunning the harvest suite failed"
fi

if [ "$TERMINATE" -eq 1 ]; then
  # Terminate instances
  printf "\nTerminating ec2 instances...\n"
  if ! ansible-playbook -i "$ANSIBLE_LOCAL_INVENTORY" "$AGENT_ROOT_DIR/test/harvest/ansible/terminate-ec2.yml"; then
    printf "\nTerminating ec2 instances failed"
    exit 1
  fi
fi

exit $EXIT_CODE
