#!/bin/bash

print_usage() {
  printf -- "Usage: %s\n" $(basename "${0}")
  printf -- "-o: Output file for generated checksums\n"
  printf -- "-p: Path to look for the files to generate checksums for\n"
  printf -- "-r: Regex to find files to generate checksum for.  e.g. -f '.*tar.gz\|.*msi'\n"
}

SEARCH_PATH='./'
FIND_REGEX='.*'

while getopts 'o:p:r:' flag
do
    if [ -z "${OPTARG}" ]; then
      continue
    fi
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

if [ -n "${OUTPUT_FILE}" ]; then
  output_file="./${OUTPUT_FILE}"
  echo -n > "${output_file}"
fi

for filename in $(eval find "${SEARCH_PATH}" -maxdepth 1 -regex "'${FIND_REGEX}'"  -type f);do
   # If output filename was not provided
   if [ -z "${OUTPUT_FILE}" ]; then
     output_file="./${filename}.sum"
     echo -n > "${output_file}"
   fi

   echo "Processing file: ${filename}, creating ${output_file}"
   sha256sum "${filename}" | awk -F ' ' '{gsub(".*/", "", $2); print $1 "  " $2}' >> "${output_file}"
done
