#!/usr/bin/env sh

echo "stdout line"
echo "error line" 1>&2
echo "${PREFIX}-$1"
