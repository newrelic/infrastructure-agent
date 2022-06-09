#!/bin/bash

print_usage() {
  printf -- "Usage: %s\n" "$(basename ${0})"
  printf -- "-o: Output file for generated checksums\n"
  printf -- "-p: Path to look for the files to generate checksums for\n"
  printf -- "-r: Regex to find files to generate checksum for.  e.g. -f '.*tar.gz\|.*msi'\n"
}

OUTPUT_FILE='checksums.txt'
SEARCH_PATH='./'
FIND_REGEX='.*'

while getopts 'o:p:r:' flag
do
    case "${flag}" in
        h)
          print_usage
          exit 0
        ;;
        o)
          OUTPUT_FILE="${OPTARG}"
          continue
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

echo -n > "${OUTPUT_FILE}"
for filename in $(eval find "${SEARCH_PATH}" -maxdepth 1 -regex "'${FIND_REGEX}'");do
   if [ "${filename}" == "${OUTPUT_FILE}" ]; then
     continue
   fi
   echo "Processing file: ${filename}"
   sha256sum "${filename}" | awk -F ' ' '{gsub(".*/", "", $2); print $1 "  " $2}' >> "${OUTPUT_FILE}"
done
