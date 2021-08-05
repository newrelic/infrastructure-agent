#!/bin/bash
set -e
#
#
# Gets dist/tarball_dirty created by Goreleaser (all files in root path) and reorganize files in correct path
#
#

for tarball_dirty in $(find dist -regex ".*darwin.*_dirty\.\(tar.gz\)");do
  tarball=${tarball_dirty:5:${#tarball_dirty}-(5+13)} # Strips begining and end chars
  tarball="${tarball}.tar.gz"
  tarballTmpPath="dist/tarball_temp"
  tarballContentPath="${tarballTmpPath}"
  tarballContentPathBin="${tarballTmpPath}/usr/local/bin/newrelic-infra"
  tarballContentPathVarDb="${tarballTmpPath}/usr/local/var/db/newrelic-infra"
  tarballContentPathEtc="${tarballTmpPath}/usr/local/etc/newrelic-infra"

  echo "===> Create ${tarballContentPathBin}/ for binaries & licence"
  mkdir -p ${tarballContentPath}/newrelic-infra/
  echo "===> Create ${tarballContentPathEtc}/ for config"
  mkdir -p ${tarballContentPathEtc}/
  echo "===> Create ${tarballContentPathVarDb}/ for data & license"
  mkdir -p ${tarballContentPathVarDb}/

  echo "===> Decompress ${tarball} in ${tarballContentPath}"
  tar -xvf ${tarball_dirty} -C ${tarballContentPath}

  echo "===> Move executable files inside ${tarball}"
  mv ${tarballContentPath}/newrelic-infra "${tarballContentPathBin}/"
  mv ${tarballContentPath}/newrelic-infra-service "${tarballContentPathBin}/"
  mv ${tarballContentPath}/newrelic-infra-ctl "${tarballContentPathBin}/"

  echo "===> Copy licence ${tarballContentPathVarDb}/LICENSE.macos.txt"
  cp assets/licence/LICENSE.macos.txt "${tarballContentPathVarDb}/LICENSE.macos.txt"

  echo "===> Creating tarball ${TARBALL_CLEAN}"
  tar -czf dist/${tarball} -C "${tarballContentPath}/" .

  echo "===> Cleaning dirty tarball ${tarball_dirty}"
  rm ${tarball_dirty}
  rm -rf ${tarballTmpPath}
done