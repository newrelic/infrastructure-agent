#!/bin/bash
set -e
#
#
# Upload dist artifacts to GH Release assets
#
#
cd dist
for filename in $(find  -regex ".*\.\(msi\|rpm\|deb\|zip\|tar.gz\)");do
  echo "===> Uploading to GH $TAG: ${filename}"
      gh release upload $TAG $filename
done