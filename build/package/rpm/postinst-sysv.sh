#!/bin/sh
# RPM and SYSV distros: RHEL 5, SUSE 11

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
    # Suse 11 kernel capabilities seem not working, so we avoid them
    suseRelease=$(cat /etc/SuSE-release 2>/dev/null | grep VERSION | awk '{print $NF}')
    setCap=$(command -v setcap) || setCap="/sbin/setcap" && [ -f $setCap ] || setCap=""
    if [ "$suseRelease" != "11" ] && [ ! -z $setCap ]; then
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

# RPM runs the prerm after installing the new package, meaning that the stop
# signal will be sent to the new pid /var/run/newrelic-infra/newrelic-infra.pid
# which old versions of the agent won't receive so we check and stopp them here.

oldPid=/var/run/newrelic-infra.pid
if [ -e "$oldPid" ] ; then
  killproc -p $oldPid /usr/bin/newrelic-infra
  rm $oldPid
fi

if [ -e "$serviceFile" ]; then
	${serviceFile} start
fi
