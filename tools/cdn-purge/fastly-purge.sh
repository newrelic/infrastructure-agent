#!/bin/bash

cd tools/cdn-purge
go mod vendor
FASTLY_KEY=${FASTLY_KEY} go run fastly-purge.go -v -b nr-downloads-main
