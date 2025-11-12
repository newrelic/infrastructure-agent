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


# Sign RPM's (excluding EL10)
echo "===> Create .rpmmacros to sign rpm's from Goreleaser"
echo "%_gpg_name ${GPG_MAIL}" >> ~/.rpmmacros
echo "%_signature gpg" >> ~/.rpmmacros
echo "%_gpg_path /root/.gnupg" >> ~/.rpmmacros
echo "%_gpgbin /usr/bin/gpg" >> ~/.rpmmacros
echo "%__gpg_sign_cmd   %{__gpg} gpg --no-verbose --no-armor --passphrase ${GPG_PASSPHRASE} --no-secmem-warning --digest-algo sha256 -u "%{_gpg_name}" -sbo %{__signature_filename} %{__plaintext_filename}" >> ~/.rpmmacros

echo "===> Importing GPG private key from GHA secrets..."
printf %s ${GPG_PRIVATE_KEY_BASE64} | base64 -d | gpg --batch --import -

echo "===> Importing GPG signature, needed from Goreleaser to verify signature"
gpg --export -a ${GPG_MAIL} > /tmp/RPM-GPG-KEY-${GPG_MAIL}
rpm --import /tmp/RPM-GPG-KEY-${GPG_MAIL}

cd dist

sles_regex="(.*sles12.*)"

for rpm_file in $(find -regex ".*\.\(rpm\)" | grep -v "el10");do
  echo "===> Signing $rpm_file"

  ../build/sign_rpm.exp $rpm_file ${GPG_PASSPHRASE}

  echo "===> Sign verification $rpm_file"
  rpm -v --checksig $rpm_file
done

# Sign EL10 RPM's with OHAI GPG key
echo "===> Create .rpmmacros for EL10 rpm's with OHAI GPG key"
echo "%_gpg_name ${GPG_MAIL}" > ~/.rpmmacros_sha256
echo "%_signature gpg" >> ~/.rpmmacros_sha256
echo "%_gpg_path /root/.gnupg" >> ~/.rpmmacros_sha256
echo "%_gpgbin /usr/bin/gpg" >> ~/.rpmmacros_sha256
echo "%__gpg_sign_cmd   %{__gpg} gpg --no-verbose --no-armor --passphrase ${GPG_PASSPHRASE} --no-secmem-warning --digest-algo sha256 -u "%{_gpg_name}" -sbo %{__signature_filename} %{__plaintext_filename}" >> ~/.rpmmacros_sha256

echo "===> Importing OHAI GPG private key for EL10 from GHA secrets..."
printf %s ${OHAI_GPG_PRIVATE_KEY_SHA256_BASE64} | base64 -d | gpg --batch --import -

echo "===> Importing OHAI GPG signature for EL10, needed from Goreleaser to verify signature"
gpg --export -a ${GPG_MAIL} > /tmp/RPM-GPG-KEY-SHA256-${GPG_MAIL}
rpm --import /tmp/RPM-GPG-KEY-SHA256-${GPG_MAIL}

# Backup original .rpmmacros and use SHA256 specific one
cp ~/.rpmmacros ~/.rpmmacros_backup
cp ~/.rpmmacros_sha256 ~/.rpmmacros

for rpm_file in $(find -regex ".*\.\(rpm\)" | grep "el10");do
  echo "===> Signing EL10 $rpm_file with OHAI GPG key"

  ../build/sign_rpm.exp $rpm_file ${GPG_PASSPHRASE}

  echo "===> Sign verification $rpm_file"
  rpm -v --checksig $rpm_file
done

# Restore original .rpmmacros
cp ~/.rpmmacros_backup ~/.rpmmacros

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