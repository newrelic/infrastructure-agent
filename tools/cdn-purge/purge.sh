#!/bin/bash

cd tools/cdn-purge
go mod vendor
CLOUDFARE_KEY=${CLOUDFARE_KEY} go run purge.go -v -b nr-downloads-main
