#!/bin/bash
set -e
#
#
# Gets dist/tarball_dirty created by Goreleaser (all files in root path) and reorganize files in correct path
#
#

for tarball_dirty in $(find dist -regex ".*linux.*_dirty\.\(tar.gz\)");do
  tarball=${tarball_dirty:5:${#tarball_dirty}-(5+13)} # Strips begining and end chars
  tarball="${tarball}.tar.gz"
  tarballTmpPath="dist/tarball_temp"
  tarballContentPath="${tarballTmpPath}/newrelic-infra"

  mkdir -p ${tarballContentPath}/etc/{newrelic-infra/integrations.d,init_scripts/{systemd,sysv,upstart}}
  mkdir -p ${tarballContentPath}/usr/bin
  mkdir -p ${tarballContentPath}/var/{db/newrelic-infra,log/newrelic-infra,run/newrelic-infra}
  mkdir -p ${tarballContentPath}/var/db/newrelic-infra/{custom-integrations,integrations.d,newrelic-integrations}
  mkdir -p ${tarballContentPath}/opt/newrelic-infra/{custom-integrations,newrelic-integrations}

  echo "===> Decompress ${tarball} in ${tarballContentPath}"
  tar -xvf ${tarball_dirty} -C ${tarballContentPath}

  echo "===> Move files inside ${tarball}"
  mv ${tarballContentPath}/newrelic-infra "${tarballContentPath}/usr/bin/"
  mv ${tarballContentPath}/newrelic-infra-service "${tarballContentPath}/usr/bin/"
  mv ${tarballContentPath}/newrelic-infra-ctl "${tarballContentPath}/usr/bin/"

  cp build/package/binaries/linux/config_defaults.sh "${tarballContentPath}/"
  cp build/package/systemd/newrelic-infra.service "${tarballContentPath}/etc/init_scripts/systemd/"
  cp build/package/sysv/deb/newrelic-infra  "${tarballContentPath}/etc/init_scripts/sysv/"
  cp build/package/upstart/newrelic-infra  "${tarballContentPath}/etc/init_scripts/upstart/"
  cp build/package/binaries/linux/installer.sh "${tarballContentPath}/"
  cp assets/examples/infrastructure/LICENSE.linux.txt "${tarballContentPath}/var/db/newrelic-infra/LICENSE.txt"

  echo "===> Creating tarball ${TARBALL_CLEAN}"
  tar -czf dist/${tarball} -C "${tarballContentPath}/../" .

  echo "===> Cleaning dirty tarball ${tarball_dirty}"
  rm ${tarball_dirty}
  rm -rf ${tarballTmpPath}
done