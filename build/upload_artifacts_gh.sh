#!/bin/bash
#
#
# Upload dist artifacts to GH Release assets
#
#


print_usage() {
  printf -- "Usage: %s\n" $(basename "${0}")
  printf -- "-p: Path to look for the files\n"
  printf -- "-r: Regex to find files e.g. -f '.*tar.gz\|.*msi'\n"
}

SEARCH_PATH='dist'
FIND_REGEX='.*\.\(msi\|rpm\|deb\|zip\|tar.gz\|sum\)'

while getopts 'p:r:' flag
do
    case "${flag}" in
        h)
          print_usage
          exit 0
        ;;
        r)
          FIND_REGEX="${OPTARG}"
          continue
        ;;
        p)
         SEARCH_PATH="${OPTARG}"
         continue
        ;;
        *)
          print_usage
          exit 1
        ;;
    esac
done

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
cd "${SEARCH_PATH}"
for filename in $(find . -regex "${FIND_REGEX}" -type f);do
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