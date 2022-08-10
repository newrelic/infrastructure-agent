#!/usr/bin/env sh
set -e

# Sign DEB's
GNUPGHOME="/root/.gnupg"
echo "${GPG_PASSPHRASE}" > "${GNUPGHOME}/gpg-passphrase"
echo "passphrase-file ${GNUPGHOME}/gpg-passphrase" >> "$GNUPGHOME/gpg.conf"
echo 'allow-loopback-pinentry' >> "${GNUPGHOME}/gpg-agent.conf"
echo 'pinentry-mode loopback' >> "${GNUPGHOME}/gpg.conf"
echo 'use-agent' >> "${GNUPGHOME}/gpg.conf"
echo RELOADAGENT | gpg-connect-agent

for deb_file in $(find -regex ".*\.\(deb\)");do
  echo "===> Signing $deb_file"
  debsigs --sign=origin --verify --check -v -k ${GPG_MAIL} $deb_file
done
