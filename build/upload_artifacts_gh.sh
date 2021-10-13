#!/bin/bash
#
#
# Upload dist artifacts to GH Release assets
#
#

delete_asset_by_name() {
  artifact="${1}"
  repo="newrelic/infrastructure-agent"

  assets_url=$(gh api "repos/${repo}/releases/tags/${TAG}" --jq '[.assets_url] | @tsv')
  if [ "${?}" -ne 0 ]; then
    exit 1
  fi

  assets=$(gh api "${assets_url}" --jq '.[] | [.url,.name] | @tsv' | tee)
  if [ "${?}" -ne 0 ]; then
    exit 2
  fi

  while IFS= read -r asset;
  do
    assetArray=($asset)
    if [ "${assetArray[1]}" = "${artifact}"  ]; then
      gh api -X DELETE "${assetArray[0]}"
      if [ "${?}" -ne 0 ]; then
        exit 3
      fi
      return
    fi
  done < <(echo "$assets")

  echo "no assets found to delete with the name: ${artifact}"
  exit 4
}

MAX_ATTEMPTS=5
ATTEMPTS=$MAX_ATTEMPTS
cd dist
for filename in $(find . -name "*.msi" -o -name "*.rpm" -o -name "*.deb" -o -name "*.zip" -o -name "*.tar.gz");do
  echo "===> Uploading to GH $TAG: ${filename}"
  while [ $ATTEMPTS -gt 0 ];do
    gh release upload $TAG $filename --clobber
    if [[ $? -eq 0 ]];then
      echo "===> uploaded  ${filename}"
      break
    fi
    set -e
      delete_asset_by_name "${filename}"
    set +e
    (( ATTEMPTS-- ))
  done
  if [ $ATTEMPTS -eq 0 ];then
    echo "too many attempts to upload $filename"
    exit 1
  fi
  ATTEMPTS=$MAX_ATTEMPTS
done