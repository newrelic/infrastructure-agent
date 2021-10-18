#!/bin/bash
#
#
# Upload dist artifacts to GH Release assets
#
#

# delete_asset_by_name is used when we want to re-upload an asset that failed or was partially published.
delete_asset_by_name() {
  artifact="${1}"
  repo="newrelic/infrastructure-agent"

  assets_url=$(gh api "repos/${repo}/releases/tags/${TAG}" --jq '[.assets_url] | @tsv')
  if [ "${?}" -ne 0 ]; then
    exit 1
  fi

  page=1
  while [ "${page}" -lt 20 ]; do
    echo "fetching assets page: ${page}..."
    assets=$(gh api "${assets_url}?page=${page}" --jq '.[] | [.url,.name] | @tsv' | tee)
    if [ "${?}" -ne 0 ]; then
      exit 2
    fi

    if [ "${assets}" = "" ]; then
      break
    fi

    while IFS= read -r asset;
    do
      assetArray=($asset)
      if [ "${assetArray[1]}" = "${artifact}"  ]; then
        gh api -X DELETE "${assetArray[0]}"
        if [ "${?}" -ne 0 ]; then
          exit 3
        fi
        echo "deleted ${artifact}, retry..."
        return
      fi
    done < <(echo "$assets")
  ((page++))
  done
  echo "no assets found to delete with the name: ${artifact}"
}

MAX_ATTEMPTS=20
ATTEMPTS=$MAX_ATTEMPTS
cd dist
for filename in $(find . -name "*.msi" -o -name "*.rpm" -o -name "*.deb" -o -name "*.zip" -o -name "*.tar.gz");do
  echo "===> Uploading to GH $TAG: ${filename}"
  while [ "${ATTEMPTS}" -gt 0 ];do
    gh release upload "${TAG}" "${filename}" --clobber
    if [[ "${?}" -eq 0 ]];then
      echo "===> uploaded  ${filename}"
      break
    fi
    set -e
      delete_asset_by_name $(basename "${filename}")
    set +e
    sleep 3s
    (( ATTEMPTS-- ))
  done
  if [ "${ATTEMPTS}" -eq 0 ];then
    echo "too many attempts to upload ${filename}"
    exit 1
  fi
  ATTEMPTS="${MAX_ATTEMPTS}"
done