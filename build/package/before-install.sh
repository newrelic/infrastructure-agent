#!/bin/sh


false=0
true=1
# The mode flag file is used to notify the post-install scripts
modeFlagFile=/tmp/nria_mode
rm $modeFlagFile >/dev/null 2>&1

# check NRIA_MODE is set
if [ -z "${NRIA_MODE}" ]; then
  NRIA_MODE="ROOT"
fi

# check the user mode provided by the user
if [ ${NRIA_MODE} != "ROOT" ] && [ ${NRIA_MODE} != "PRIVILEGED" ] && [ ${NRIA_MODE} != "UNPRIVILEGED" ]; then
  # user mode is not valid so we set it by default: ROOT
  NRIA_MODE="ROOT"
fi

if [ ${NRIA_MODE} != "ROOT" ]; then
  # create nri-agent group and/or user if they don't already exist
  groupAdd=$(command -v groupadd) || groupAdd="/usr/sbin/groupadd"
  getent group nri-agent >/dev/null || eval "$groupAdd -r nri-agent" || exit 1
  id -u nri-agent >/dev/null 2>&1
  userAlreadyExists=$?
  if [ $userAlreadyExists -ne 0 ]; then
    noLogin=$(command -v nologin) || noLogin="/usr/sbin/nologin" && [ -f $noLogin ] || noLogin="/sbin/nologin"
    userAdd=$(command -v useradd) || userAdd="/usr/sbin/useradd"
    eval "$userAdd -r -d / --shell $noLogin -g nri-agent nri-agent" || exit 1
  elif id -nG nri-agent | grep --invert-match --word-regexp --quiet 'nri-agent'; then
    # User exists but is not part of the nri-agent group
    userMod=$(command -v usermod) || userMod="/usr/sbin/usermod"
    eval "$userMod -g nri-agent nri-agent" || exit 1
  fi
fi

# is this an update or is it a fresh install?
infra=/usr/bin/newrelic-infra
$(test -f $infra)
if [ $? -eq 0 ]; then
  # this is an update, so we need to discover the current mode.
  # Check if there is a previous init file set with the nri-agent user
  nonRootUser=${false}
  if command -v systemctl >/dev/null 2>&1; then
    serviceFile=/etc/systemd/system/newrelic-infra.service
    if [ -e $serviceFile ] && grep "User=nri-agent" $serviceFile ; then
      nonRootUser=${true}
    fi
  elif command -v initctl >/dev/null 2>&1; then
    serviceFile=/etc/init/newrelic-infra.conf
    if [ -e $serviceFile ] && grep -e "exec \"\$0\" \"\$@\"' nri-agent --" $serviceFile >/dev/null 2>&1; then
      nonRootUser=${true}
    fi
  elif command -v update-rc.d >/dev/null 2>&1; then
    serviceFile=/etc/init.d/newrelic-infra
    if [ -e $serviceFile ]  && grep "User=nri-agent" $serviceFile ; then
      nonRootUser=${true}
    fi
  else
    # Some sysvinit systems (sles11sp4) don't have update-rc.d
    serviceFile=/etc/init.d/newrelic-infra
    if [ -e $serviceFile ]  && grep "User=nri-agent" $serviceFile ; then
      nonRootUser=${true}
    fi
  fi

  if [ ${nonRootUser} -eq ${true} ]; then
    getCap=$(command -v getcap) || getCap="/sbin/getcap" && [ -f $getCap ] || getCap="/usr/sbin/getcap" && [ -f $getCap ] || getCap=""
    if [ ! -z $getCap ]; then
        eval "$getCap $infra" | grep -e "cap_dac_read_search" | grep -e "cap_sys_ptrace"
    fi
    if [ $? -eq 0 ]; then
      # we have kernel capabilities
      NRIA_MODE="PRIVILEGED"
    else
      NRIA_MODE="UNPRIVILEGED"
    fi
  fi
fi

# create a file to use as flag to let the post-install script know the user mode
echo ${NRIA_MODE} > $modeFlagFile