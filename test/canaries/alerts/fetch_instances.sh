#!/usr/bin/env bash

# This helps in debugging your scripts. TRACE=1 ./script.sh
if [[ "${TRACE-0}" == "1" ]]; then set -o xtrace; fi

usage() {
 echo 'Usage: MACSTADIUM_USER=test MACSTADIUM_PASSWORD=jkfsdl ./fetch_instances.sh 1.49.0 1.48.5'
}

CURRENT_VERSION=$1
PREVIOUS_VERSION=$2
if [ -z "${CURRENT_VERSION}" ];then
  { usage; echo "CURRENT_VERSION is not provided"; exit 1; }
fi
if [ -z "${PREVIOUS_VERSION}" ];then
  { usage; echo "PREVIOUS_VERSION is not provided"; exit 1; }
fi
if [ -z "${MACSTADIUM_USER}" ];then
  { usage; echo "MACSTADIUM_USER is not provided"; exit 1; }
fi
if [ -z "${MACSTADIUM_PASSWORD}" ];then
  { usage; echo "MACSTADIUM_PASSWORD is not provided"; exit 1; }
fi

INSTANCES=$( aws ec2 describe-instances --no-paginate \
  --filters 'Name=instance-state-code,Values=16' 'Name=tag:Name,Values=canary:*' \
  --query 'Reservations[*].Instances[*].{PrivateIpAddress:PrivateIpAddress,Name:[Tags[?Key==`Name`].Value][0][0],InstanceId:InstanceId}' \
  | jq -c '.[]' \
  | tr -d "[]{}\""
)

MACOS_INSTANCES=$(curl -s -H 'Accept: application/json' -H 'Content-Type: application/json' -X GET -u "$MACSTADIUM_USER:$MACSTADIUM_PASSWORD" https://api.macstadium.com/core/api/servers \
  | jq -c '.[]' \
  | jq '.name' \
)

cat << EOF
linux_display_names  = [
EOF
for ins in $INSTANCES;do
  NAME=$( echo $ins | awk -F "," '{print $2}' | sed "s/^Name://g" )

  if [[ ! "${NAME}" =~ $CURRENT_VERSION ]]; then
    continue
  fi

  if [[ "${NAME}" =~ windows ]]; then
    continue
  fi

  echo "    { previous = \"$NAME-previous\", current = \"$NAME-current\"},"

done

cat << EOF
]
EOF

cat << EOF
windows_display_names = [
EOF
for ins in $INSTANCES;do
  NAME=$( echo $ins | awk -F "," '{print $2}' | sed "s/^Name://g" )

  if [[ ! "${NAME}" =~ $CURRENT_VERSION ]]; then
    continue
  fi

  if [[ "${NAME}" =~ windows ]]; then
    MACHINE=$( echo $NAME | awk -F ":" '{print $3}')
    for ins2 in $INSTANCES;do
      NAME_PREVIOUS=$( echo $ins2 | awk -F "," '{print $2}' | sed "s/^Name://g" )
      if [[ ! "${NAME_PREVIOUS}" =~ $MACHINE || ! "${NAME_PREVIOUS}" =~ $PREVIOUS_VERSION ]]; then
        continue
      fi
      echo "    { previous = \"$NAME_PREVIOUS\", current = \"$NAME\"},"
    done
  fi
done

cat << EOF
]
EOF

cat << EOF
macos_display_names = [
EOF
for name in $MACOS_INSTANCES;do
  if [[ "${name}" =~ "current" ]]; then
    PREVIOUS_MACHINE=$( echo $name | awk -F ":" '{print $3":"$4}')
    for name_previous in $MACOS_INSTANCES;do
      if [[ "${name_previous}" =~ $PREVIOUS_MACHINE  && "${name_previous}" =~ "previous" ]]; then
        NAME_CURRENT=$(echo $name | sed "s/current/$CURRENT_VERSION/" | sed "s/canary/canary-macos/")
        NAME_PREVIOUS=$(echo $name_previous | sed "s/previous/$PREVIOUS_VERSION/" | sed "s/canary/canary-macos/")
        echo "    { previous = $NAME_PREVIOUS, current = $NAME_CURRENT},"
      fi
    done
  fi
done

cat << EOF
]
EOF
