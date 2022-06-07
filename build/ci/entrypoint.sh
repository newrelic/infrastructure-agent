#!/bin/bash
target=$1

echo "$SSH_KEY" | base64 --decode > ~/.ssh/caos-dev-arm.cer
chmod 600  ~/.ssh/caos-dev-arm.cer
git checkout $REF
ANSIBLE_INVENTORY=$ANSIBLE_INVENTORY make $target