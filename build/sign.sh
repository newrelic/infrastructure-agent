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
echo "%__gpg_sign_cmd   %{__gpg} gpg --no-verbose --no-armor --passphrase ${GPG_PASSPHRASE} --no-secmem-warning -u "%{_gpg_name}" -sbo %{__signature_filename} %{__plaintext_filename}" >> ~/.rpmmacros

echo "===> Importing GPG private key from GHA secrets..."
printf %s ${GPG_PRIVATE_KEY_BASE64} | base64 -d | gpg --batch --import -

echo "===> Importing GPG signature, needed from Goreleaser to verify signature"
gpg --export -a ${GPG_MAIL} > /tmp/RPM-GPG-KEY-${GPG_MAIL}
rpm --import /tmp/RPM-GPG-KEY-${GPG_MAIL}

cd dist

sles_regex="(.*sles12.*)"

for rpm_file in $(find -regex ".*\.\(rpm\)");do
  echo "===> Signing $rpm_file"

  # if suse 12.x, then add --rpmv3
  if [[ $rpm_file =~ $sles_regex ]]; then
     rpmsign -v --addsign --rpmv3 $rpm_file
  else
    ../build/sign.exp $rpm_file ${GPG_PASSPHRASE}
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

for deb_file in $(find -regex ".*\.\(deb\)");do
  echo "===> Signing $deb_file"
  debsigs --sign=origin --verify --check -v -k ${GPG_MAIL} $deb_file
done

# Make sure the sign_tar.exp script is executable
chmod +x ../build/sign_tar.exp

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