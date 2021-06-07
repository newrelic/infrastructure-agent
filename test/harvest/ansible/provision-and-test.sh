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

# Provision ec2 instances
ANSIBLE_INVENTORY="$( pwd )/test/harvest/ansible/inventory.local"
printf "\nProvisioning ec2 instances...\n"
if ! ansible-playbook -i "$ANSIBLE_INVENTORY" test/harvest/ansible/provision-ec2.yml; then
  printf "\nProvisioning ec2 instances failed"
  exit 1
fi

# Build and run harvest tests in provisioned instances
HARVEST_TESTS_ANSIBLE_INVENTORY="$( pwd )/test/harvest/ansible/inventory.ec2"
AGENT_ROOT_DIR=$( pwd )
# Ensure GOBIN is empty to avoid "cannot install cross-compiled binaries when GOBIN is set"
export GOBIN=""
printf "\nTesting initial install...\n"
if ! ansible-playbook -i "$HARVEST_TESTS_ANSIBLE_INVENTORY" -e agent_root_dir="$AGENT_ROOT_DIR" test/harvest/ansible/test.yml; then
  printf "\nRunning the harvest suite failed"
  exit 1
fi

# Terminate instances
printf "\nTerminating ec2 instances...\n"
if ! ansible-playbook -i "$ANSIBLE_INVENTORY" test/harvest/ansible/terminate-ec2.yml; then
  printf "\nTerminating ec2 instances failed"
  exit 1
fi

