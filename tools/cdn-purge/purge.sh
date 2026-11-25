#!/bin/bash

# Bucket served by the CDN; defaults to the legacy production bucket.
BUCKET=${AWS_S3_BUCKET_NAME:-nr-downloads-main}

cd tools/cdn-purge
go mod vendor
CLOUDFARE_KEY=${CLOUDFARE_KEY} go run purge.go -v -b "${BUCKET}" -r "${AWS_REGION:-us-east-1}" -p "${REPO_PATH:-infrastructure_agent}"
