#!/usr/bin/env sh
set -e
#
#
#
# Sign RPM's & DEB's in /dist artifacts to GH Release Assets
#
#
#
# Function to start gpg-agent if not running
start_gpg_agent() {
    if ! pgrep -x "gpg-agent" > /dev/null
    then
        echo "Starting gpg-agent..."
        eval $(gpg-agent --daemon)
    else
        echo "gpg-agent is already running."
    fi
}

# Ensure gpg-agent is running
start_gpg_agent

# Import GPG private key from GHA secrets (needed for DEB and TARGZ signing)
printf %s ${GPG_PRIVATE_KEY_BASE64} | base64 -d | gpg --batch --import -

cd dist

# Sign DEB's
GNUPGHOME="/root/.gnupg"
echo "${GPG_PASSPHRASE}" > "${GNUPGHOME}/gpg-passphrase"
echo "passphrase-file ${GNUPGHOME}/gpg-passphrase" >> "$GNUPGHOME/gpg.conf"
# echo 'allow-loopback-pinentry' >> "${GNUPGHOME}/gpg-agent.conf"
# echo 'pinentry-mode loopback' >> "${GNUPGHOME}/gpg.conf"
echo 'use-agent' >> "${GNUPGHOME}/gpg.conf"
echo RELOADAGENT | gpg-connect-agent

for deb_file in $(find -regex ".*\.\(deb\)"); do
  echo "===> Signing $deb_file"

  # Run the sign_deb.exp script to sign the .deb file
  ../build/sign_deb.exp $deb_file ${GPG_PASSPHRASE} ${GPG_MAIL}

  echo "===> Sign verification $deb_file"
  dpkg-sig --verify $deb_file
done

# Sign TARGZ files
for targz_file in $(find . -type f -name "*.tar.gz"); do
  echo "===> Signing $targz_file"
  ../build/sign_tar.exp $targz_file ${GPG_PASSPHRASE}
  asc_file="${targz_file}.asc"
  if [ -f "$asc_file" ]; then
    echo "===> Sign verification $targz_file"
    gpg --verify "$asc_file" "$targz_file"
  else
    echo "Error: Signature file $asc_file not found."
  fi
done



if [ -n "${GPG_PRIVATE_KEY_SHA256_BASE64}" ]; then
    if [ -n "${GPG_MAIL}" ]; then
      EXISTING_KEY_ID=$(gpg --list-secret-keys --with-colons "${GPG_MAIL}" | grep '^sec:' | tail -1 | cut -d: -f5)
      if [ -n "$EXISTING_KEY_ID" ]; then
        echo "===> Deleting existing GPG key: $EXISTING_KEY_ID"
         gpg --batch --yes --delete-secret-keys "$EXISTING_KEY_ID" 2>/dev/null || true
        gpg --batch --yes --delete-keys "$EXISTING_KEY_ID" 2>/dev/null || true
      fi
    fi

    echo "===> Importing SHA256 GPG private key from GHA secrets..."
    printf %s ${GPG_PRIVATE_KEY_SHA256_BASE64} | base64 -d | gpg --batch --import -
fi


# Sign RPM's
echo "===> Create .rpmmacros to sign rpm's from Goreleaser"
echo "%_gpg_name ${GPG_MAIL}" >> ~/.rpmmacros
echo "%_signature gpg" >> ~/.rpmmacros
echo "%_gpg_path /root/.gnupg" >> ~/.rpmmacros
echo "%_gpgbin /usr/bin/gpg" >> ~/.rpmmacros
echo "%__gpg_sign_cmd   %{__gpg} gpg --no-verbose --no-armor --passphrase ${GPG_PASSPHRASE} --no-secmem-warning --digest-algo sha256 -u "%{_gpg_name}" -sbo %{__signature_filename} %{__plaintext_filename}" >> ~/.rpmmacros

echo "===> Importing GPG signature, needed from Goreleaser to verify signature"
gpg --export -a ${GPG_MAIL} > /tmp/RPM-GPG-KEY-${GPG_MAIL}
rpm --import /tmp/RPM-GPG-KEY-${GPG_MAIL}

sles_regex="(.*sles12.*)"

for rpm_file in $(find -regex ".*\.\(rpm\)");do
  echo "===> Signing $rpm_file"

  ../build/sign_rpm.exp $rpm_file ${GPG_PASSPHRASE}

  echo "===> Sign verification $rpm_file"
  rpm -v --checksig $rpm_file
done
