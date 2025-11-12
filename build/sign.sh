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


# Sign RPM's
echo "===> Create .rpmmacros to sign rpm's from Goreleaser"
echo "%_gpg_name ${GPG_MAIL}" >> ~/.rpmmacros
echo "%_signature gpg" >> ~/.rpmmacros
echo "%_gpg_path /root/.gnupg" >> ~/.rpmmacros
echo "%_gpgbin /usr/bin/gpg" >> ~/.rpmmacros
echo "%__gpg_sign_cmd   %{__gpg} gpg --no-verbose --no-armor --passphrase ${GPG_PASSPHRASE} --no-secmem-warning --digest-algo sha256 -u "%{_gpg_name}" -sbo %{__signature_filename} %{__plaintext_filename}" >> ~/.rpmmacros

echo "===> Importing GPG private key from GHA secrets..."
# We'll import the appropriate key for each package type in the loop

echo "===> Importing GPG signature, needed from Goreleaser to verify signature"
gpg --export -a ${GPG_MAIL} > /tmp/RPM-GPG-KEY-${GPG_MAIL}
rpm --import /tmp/RPM-GPG-KEY-${GPG_MAIL}

cd dist

sles_regex="(.*sles12.*)"

for rpm_file in $(find -regex ".*\.\(rpm\)");do
  echo "===> Signing $rpm_file"

  # Check if this is an el10 RPM file
  if echo "$rpm_file" | grep -q "el10"; then
    echo "===> el10 RPM detected, using OHAI GPG key"
    # Clear GPG keyring and import only OHAI key
    gpg --batch --yes --delete-secret-keys ${GPG_MAIL} 2>/dev/null || true
    printf %s ${OHAI_GPG_PRIVATE_KEY_SHA256_BASE64} | base64 -d | gpg --batch --import -
    ../build/sign_rpm.exp $rpm_file ${GPG_PASSPHRASE}
  else
    echo "===> Non-el10 RPM detected, using regular GPG key"
    # Clear GPG keyring and import only regular key
    gpg --batch --yes --delete-secret-keys ${GPG_MAIL} 2>/dev/null || true
    printf %s ${GPG_PRIVATE_KEY_BASE64} | base64 -d | gpg --batch --import -
    ../build/sign_rpm.exp $rpm_file ${GPG_PASSPHRASE}
  fi

  echo "===> Sign verification $rpm_file"
  rpm -v --checksig $rpm_file
done

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