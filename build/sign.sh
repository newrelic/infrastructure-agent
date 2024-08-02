#!/usr/bin/env sh
set -e
#
#
#
# Sign RPM's & DEB's in /dist artifacts to GH Release Assets
#
#
#

# Set gpg-agent defaults
GNUPGHOME="/root/.gnupg"

echo "${GPG_PASSPHRASE}" > "${GNUPGHOME}/gpg-passphrase"
echo "passphrase-file ${GNUPGHOME}/gpg-passphrase" >> "$GNUPGHOME/gpg.conf"
echo 'allow-loopback-pinentry' >> "${GNUPGHOME}/gpg-agent.conf"
echo 'pinentry-mode loopback' >> "${GNUPGHOME}/gpg.conf"
echo 'use-agent' >> "${GNUPGHOME}/gpg.conf"
echo RELOADAGENT | gpg-connect-agent

# Sign RPM's
echo "===> Create .rpmmacros to sign rpm's from Goreleaser"
echo "%_gpg_name ${GPG_MAIL}" >> ~/.rpmmacros
echo "%_signature gpg" >> ~/.rpmmacros
echo "%_gpg_path /root/.gnupg" >> ~/.rpmmacros
echo "%__gpgbin /usr/bin/gpg2" >> ~/.rpmmacros
echo "%__gpg_sign_cmd   %{__gpg} gpg --verbose --no-armor --yes --batch --pinentry-mode loopback --passphrase ${GPG_PASSPHRASE} --no-secmem-warning -u "%{_gpg_name}" -sbo %{__signature_filename} %{__plaintext_filename}" >> ~/.rpmmacros

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
    rpmsign -v --addsign $rpm_file
  fi

  echo "===> Sign verification $rpm_file"
  rpm -v --checksig $rpm_file
done


for deb_file in $(find -regex ".*\.\(deb\)");do
  echo "===> Signing $deb_file"
  debsigs --sign=origin --verify --check -v -k ${GPG_MAIL} $deb_file
done

# Sign TARGZ files
for targz_file in $(find -regex ".*\.\(tar.gz\)");do
  echo "===> Signing $targz_file"
  gpg --sign --armor --detach-sig $targz_file
  echo "===> Sign verification $targz_file"
  gpg --verify ${targz_file}.asc $targz_file
done
