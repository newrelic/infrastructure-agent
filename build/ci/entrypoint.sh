#!/bin/bash
target=$1

echo "$SSH_KEY" | base64 --decode > ~/.ssh/caos-dev-arm.cer
chmod 600  ~/.ssh/caos-dev-arm.cer
git fetch origin
git checkout $REF
mkdir -p $ANSIBLE_INVENTORY_FOLDER
ANSIBLE_INVENTORY=$ANSIBLE_INVENTORY $target
