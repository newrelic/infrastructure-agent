#!/bin/bash

print_usage() {
  printf -- "Usage: %s\n" "$(basename ${0})"
  printf -- "-o: Output file\n"
  printf -- "-p: Path to look for the files to be merged\n"
  printf -- "-r: Regex to find files to be merged\n"
}

OUTPUT_FILE='file.txt'
SEARCH_PATH='./'
FIND_REGEX='.*txt'

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
for filename in $(eval find "${SEARCH_PATH}" -type f -maxdepth 1 -regex "'${FIND_REGEX}'");do
   echo "Merging file: ${filename}"
   cat "${filename}" >> "${OUTPUT_FILE}"
done
