#!/bin/bash
#
#
# Upload dist artifacts to GH Release assets
#
#
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
    (( ATTEMPTS-- ))
  done
  if [ $ATTEMPTS -eq 0 ];then
    echo "too many attempts to upload $filename"
    exit 1
  fi
  ATTEMPTS=$MAX_ATTEMPTS
done