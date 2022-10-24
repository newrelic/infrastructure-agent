#!/usr/bin/env bash

#create infra agent ansible inventory with ec2 instances matching PATTERN
PATTERN=$1
if [ -z "${PATTERN}" ];then
  echo "pattern cannot be empty"
  exit 1
fi

INSTANCES=$( aws ec2 describe-instances --no-paginate \
  --filters 'Name=instance-state-code,Values=16' \
  --query 'Reservations[*].Instances[*].{PrivateIpAddress:PrivateIpAddress,Name:[Tags[?Key==`Name`].Value][0][0],InstanceId:InstanceId}' \
  | jq -c '.[]' \
  | tr -d "[]{}\""

)

cat << EOF

[localhost]
localhost ansible_connection=local

[testing_hosts:children]
linux_amd64
linux_arm64
windows_amd64
macos_current
macos_previous

[testing_hosts_linux:children]
linux_amd64
linux_arm64

[testing_hosts_windows:children]
windows_amd64

[testing_hosts_macos:children]
macos_current
macos_previous

[linux_amd64]
EOF
declare -a PYTHON2_INSTANCES=("sles-12.4" "centos7" "sles-12.5" "debian-jessie" "redhat-7.6" "redhat-7.9" "al-2$")
for ins in $INSTANCES;do

  NAME=$( echo $ins | awk -F "," '{print $2}' | sed "s/^Name://g" )
  IID=$( echo $ins | awk -F "," '{print $3}' | sed "s/^InstanceId://g" )


  PRIVATE_IP=$( echo $ins | awk -F "," '{print $1}' | sed "s/^PrivateIpAddress://g")

  if [[ "${NAME}" =~ windows ]]; then
    continue
  fi

  if [[ ! "${NAME}" =~ amd ]]; then
    continue
  fi

  if [[ ! "${NAME}" =~ $PATTERN ]]; then
    continue
  fi

  #
  GOOD_NAME=$( echo $NAME | sed "s/ubuntu-runner//g" )
  if [[ "${GOOD_NAME}" =~ ubuntu ]]; then
    USERNAME="ubuntu"
  elif [[ "${GOOD_NAME}" =~ debian ]]; then
    USERNAME="admin"
  elif [[ "${GOOD_NAME}" =~ centos ]]; then
    USERNAME="centos"
  else
    USERNAME="ec2-user"
  fi

  PYTHON="/usr/bin/python3"
  for PY2INS in "${PYTHON2_INSTANCES[@]}";do
    if [[ "${NAME}" =~ ${PY2INS} ]]; then
      PYTHON="/usr/bin/python"
      break
    fi
  done
  echo "$NAME ansible_host=$PRIVATE_IP ansible_user=$USERNAME ansible_python_interpreter=$PYTHON iid=$IID platform=linux"
done

cat << EOF

[linux_arm64]
EOF

declare -a PYTHON2_INSTANCES=("sles-12.4" "centos7" "sles-12.5" "debian-jessie" "redhat-7.6" "redhat-7.9" "al-2$")
for ins in $INSTANCES;do

  NAME=$( echo $ins | awk -F "," '{print $2}' | sed "s/^Name://g" )
  PRIVATE_IP=$( echo $ins | awk -F "," '{print $1}' | sed "s/^PrivateIpAddress://g")
  IID=$( echo $ins | awk -F "," '{print $3}' | sed "s/^InstanceId://g" )

  if [[ "${NAME}" =~ windows ]]; then
    continue
  fi

  if [[ ! "${NAME}" =~ arm ]]; then
    continue
  fi

  if [[ ! "${NAME}" =~ $PATTERN ]]; then
    continue
  fi

  GOOD_NAME=$( echo $NAME | sed "s/ubuntu-runner//g" )
  if [[ "${GOOD_NAME}" =~ ubuntu ]]; then
    USERNAME="ubuntu"
  elif [[ "${GOOD_NAME}" =~ debian ]]; then
    USERNAME="admin"
  elif [[ "${GOOD_NAME}" =~ centos ]]; then
    USERNAME="centos"
  else
    USERNAME="ec2-user"
  fi

  PYTHON="/usr/bin/python3"
  for PY2INS in "${PYTHON2_INSTANCES[@]}";do
    if [[ "${NAME}" =~ ${PY2INS} ]]; then
      PYTHON="/usr/bin/python"
      break
    fi
  done
  echo "$NAME ansible_host=$PRIVATE_IP ansible_user=$USERNAME ansible_python_interpreter=$PYTHON iid=$IID platform=linux"
done

cat << EOF

[windows_amd64]
EOF
for ins in $INSTANCES;do

  NAME=$( echo $ins | awk -F "," '{print $2}' | sed "s/^Name://g" )
  PRIVATE_IP=$( echo $ins | awk -F "," '{print $1}' | sed "s/^PrivateIpAddress://g")

  if [[ ! "${NAME}" =~ windows ]]; then
    continue
  fi

  if [[ ! "${NAME}" =~ $PATTERN ]]; then
    continue
  fi

  echo "$NAME ansible_host=$PRIVATE_IP iid=$IID platform=windows"
done

cat << EOF

[macos_current]
EOF
for ins in $INSTANCES;do

  NAME=$( echo $ins | awk -F "," '{print $2}' | sed "s/^Name://g" )
  PRIVATE_IP=$( echo $ins | awk -F "," '{print $1}' | sed "s/^PrivateIpAddress://g")

  if [[ ! "${NAME}" =~ macos ]]; then
    continue
  fi

  if [[ ! "${NAME}" =~ $PATTERN ]]; then
    continue
  fi

  echo "$NAME ansible_host=$PRIVATE_IP platform=macos"
done

cat << EOF

[linux_amd64:vars]
ansible_ssh_private_key_file=~/.ssh/caos-dev-arm.cer
ansible_ssh_common_args='-o StrictHostKeyChecking=no'

[linux_arm64:vars]
ansible_ssh_private_key_file=~/.ssh/caos-dev-arm.cer
ansible_ssh_common_args='-o StrictHostKeyChecking=no'

[windows_amd64:vars]
ansible_winrm_transport=basic
ansible_user=ansible
ansible_password=FILL_ME
ansible_connection=winrm
ansible_winrm_server_cert_validation=ignore
ansible_winrm_scheme=https
ansible_port=5986

[macos_current:vars]
ansible_ssh_private_key_file=~/.ssh/caos-dev-arm.cer
ansible_ssh_common_args='-o StrictHostKeyChecking=no'

[macos_previous:vars]
ansible_ssh_private_key_file=~/.ssh/caos-dev-arm.cer
ansible_ssh_common_args='-o StrictHostKeyChecking=no'

EOF
