#!/bin/sh
# DEB and SYSV distros: Debian 7 'Wheezy'

serviceFile=/etc/init.d/newrelic-infra

# check the run mode
userMode=$(cat /tmp/nria_mode 2>/dev/null)

# check usermode is set
if [ -z "$userMode" ]; then
  userMode="ROOT"
fi

# check the user mode
if [ "$userMode" != "ROOT" ] && [ "$userMode" != "PRIVILEGED" ] && [ "$userMode" != "UNPRIVILEGED" ]; then
  # user mode is not valid so we set it by default: ROOT
  userMode="ROOT"
fi

if [ "$userMode" = "PRIVILEGED" ] || [ "$userMode" = "UNPRIVILEGED" ]; then
  runDir=/var/run/newrelic-infra
  installDir=/var/db/newrelic-infra
  logDir=/var/log/newrelic-infra
  configDir=/etc/newrelic-infra
  tmpDir=/tmp/nr-integrations

  # Give nri-agent ownership over it's folder
  chown -R nri-agent:nri-agent ${runDir}
  chown -R nri-agent:nri-agent ${installDir}
  chown -R nri-agent:nri-agent ${logDir}
  chown -R nri-agent:nri-agent ${configDir}
  chown -R nri-agent:nri-agent ${tmpDir} 2>/dev/null || true

  if [ "$userMode" = "PRIVILEGED" ]; then
    failFlag=0
    # Give the Agent kernel capabilities if setcap command exists
    setCap=$(command -v setcap) || setCap="/sbin/setcap" && [ -f $setCap ] || setCap=""
    if [ ! -z $setCap ]; then
      eval "$setCap CAP_SYS_PTRACE,CAP_DAC_READ_SEARCH=+ep /usr/bin/newrelic-infra" || failFlag=1
    else
      failFlag=1
    fi

    if [ $failFlag -eq 1 ]; then
      (>&2 echo "Error setting PRIVILEGED mode. Fallbacking to UNPRIVILEGED mode")
    fi
  fi

  if [ -e "$serviceFile" ]; then
    # If the user or group is set to root, change it to nri-agent
    # If no user or group is set, set it to nri-agent
    if grep 'USER=root' $serviceFile >/dev/null ; then
      sed -i 's/USER=root/USER=nri-agent/g' "$serviceFile"
    elif ! grep 'USER=' $serviceFile >/dev/null ; then
        sed -i '/### END INIT INFO/aUSER=nri-agent' "$serviceFile"
    fi
  fi
fi

# Previous versions had an incorrect `prerm` that didn't stop the service
# because it couldn't detect it was running, for that reason we have to make
# sure that there is not an older version running.

oldPid=/var/run/newrelic-infra.pid
if [ -e "$oldPid" ] ; then
  . /lib/lsb/init-functions
  killproc -p $oldPid /usr/bin/newrelic-infra
  rm $oldPid
fi

if [ -e "$serviceFile" ]; then
	insserv newrelic-infra || exit $?
	${serviceFile} start || exit $?
fi
