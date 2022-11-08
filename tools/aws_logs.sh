#!/bin/bash

# Helper script to fetch all pages of a log stream from AWS CloudWatch

tail=false
outputFile="out"

die() { print_usage; echo "$*" >&2; exit 2; }  # Fail with stderr message
needs_arg() { if [ -z "$OPTARG" ]; then die "Missing arg for --$OPT option"; fi; }

print_usage() {
  printf -- "Usage: %s\n" $(basename "${0}")
  printf -- "-l --list=<count>:               List last <count> streams for the group name ordered by latest usage desc\n"
  printf -- "-g --group-name=<group name>:    AWS Log Group Name\n"
  printf -- "-s --stream-name=<stream name>:  AWS Log Stream Name\n"
  printf -- "-t --tail:                       Tail the logs stream and output to stdout\n"
  printf -- "-o --output-file=<file name>:    File where to output\n"
  printf -- "-h --help:                       Help page\n"
}

while getopts l:g:o:s:-:th OPT; do
  if [ "$OPT" = "-" ]; then   # long option: reformulate OPT and OPTARG
    OPT="${OPTARG%%=*}"       # extract long option name
    OPTARG="${OPTARG#$OPT}"   # extract long option argument (may be empty)
    OPTARG="${OPTARG#=}"      # if long option argument, remove assigning `=`
  fi
  case "$OPT" in
    g | group-name )     needs_arg; groupName="$OPTARG" ;;
    s | stream-name )    needs_arg; streamName="$OPTARG" ;;
    o | output-file )    needs_arg; outputFile="$OPTARG" ;;
    l | list )           needs_arg; listSize="$OPTARG" ;;
    t | tail )           tail=true ;;
    h | help )     print_usage; exit 0 ;;
    ??* )          die "Illegal option --$OPT" ;;  # bad long option
    ? )            exit 2 ;;  # bad short option (error reported via getopts)
  esac
done
shift $((OPTIND-1)) # remove parsed options and args from $@ list

# Validate parameters
if [[ "${groupName}" == "" ]]; then
    die "Missing -g --group-name option"
fi

# Output only the available streams
if [[ "${listSize}" -gt 0 ]]; then
    result="$(aws logs describe-log-streams --limit ${listSize} --log-group-name ${groupName} --order-by LastEventTime --descending)"
    status=$?
    echo "${result}" | jq -r '.logStreams[] | [.logStreamName, (.creationTime/1000 | 'todate')] | @tsv | .'
    exit "${status}"
fi

if [[ "${streamName}" == "" ]]; then
    die "Missing -s --stream-name option"
fi

# Clean the output file
if [[ "${tail}" == false ]]; then
  > "${outputFile}"
fi

paginationArg="--start-from-head"
while : ; do
    if [[ "${nextToken}" != "" ]]; then
        paginationArg="--next-token ${nextToken}"
    fi

    # Query logs and extract the next page token
    result="$(aws logs get-log-events ${paginationArg} --log-group-name ${groupName} --log-stream-name ${streamName} --output json)"
    newToken=$(echo "${result}" | jq -r '.nextForwardToken')

    # If the next page token hasn't changed then no more logs are available.
    # Exit the loop only if not in tail mode
    if [[ "${newToken}" == "${nextToken}" && "${tail}" == false ]]; then
        break
    fi

    result=$(echo "${result}" | jq -r '.events[] | [(.timestamp/1000 | 'todate'), .message] | @tsv | .')

    if [[ "${tail}" == "true" ]]; then
        # In tail mode we print results to stdout
        [[  "${result}" != ""  ]] && printf -- "%s\n" "${result}"
    else
        echo "Writing logs page to file: ${newToken}"
        echo "${result}" >> "${outputFile}"
    fi

    # aws logs get-log-events can return empty results while
    # there are more log events available through the current token.
    if [[ "${result}" != "" || "${tail}" == false ]]; then
        # If you have reached the end of the stream, it returns the same token you passed in.
        nextToken="${newToken}"
    fi
done
