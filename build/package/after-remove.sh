#!/bin/sh

runDir=/var/run/newrelic-infra
installDir=/var/db/newrelic-infra
logDir=/var/log/newrelic-infra
configDir=/etc/newrelic-infra

case "$1" in
  purge)
    id -u nri-agent >/dev/null 2>&1
    userExists=$?

    if [ $userExists -eq 0 ]; then
      # Delete both User and Group
      userDel=$(command -v userdel) || userDel="/usr/sbin/userdel"
      id -u nri-agent >/dev/null 2>&1 && eval "$userDel nri-agent >/dev/null 2>&1" || true
      groupDel=$(command -v groupdel) || groupDel="/usr/sbin/groupdel"
      getent group nri-agent >/dev/null 2>&1 && eval "$groupDel nri-agent >/dev/null 2>&1" || true
    fi

    # dpkg does not remove non empty directories
    rm -rf ${runDir}
    rm -rf ${installDir}
    rm -rf ${logDir}
    rm -rf ${configDir}
  ;;
  *)
  ;;
esac
