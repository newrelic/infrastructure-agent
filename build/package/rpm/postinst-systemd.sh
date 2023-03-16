#!/bin/sh

serviceFile=/etc/systemd/system/newrelic-infra.service
oldPid=/var/run/newrelic-infra.pid
userMode=$(cat /tmp/nria_mode 2>/dev/null)

# check usermode is set
if [ -z "$userMode" ]; then
  userMode="ROOT"
fi

# check the user mode
if [ "$userMode" != "ROOT" ] && [ "$userMode" != "PRIVILEGED" ] && [ "$userMode" != "UNPRIVILEGED" ]; then
  # user mode is not valid so we set it by default: PRIVILEGED
  userMode="ROOT"
fi

if [ "$userMode" = "PRIVILEGED" ] || [ "$userMode" = "UNPRIVILEGED" ]; then
  runDir=/var/run/newrelic-infra
  installDir=/var/db/newrelic-infra
  logDir=/var/log/newrelic-infra
  configDir=/etc/newrelic-infra
  tmpDir=/tmp/nr-integrations
  binPath=/usr/bin/newrelic-infra

   # Give nri-agent ownership over its folder
  chown -R nri-agent:nri-agent ${runDir}
  chown -R nri-agent:nri-agent ${configDir}
  chown -R nri-agent:nri-agent ${logDir}
  chown -R nri-agent:nri-agent ${installDir}
  chown -R nri-agent:nri-agent ${tmpDir} 2>/dev/null || true
  chown -R nri-agent:nri-agent ${binPath}

  if [ "$userMode" = "PRIVILEGED" ]; then
    # Give the Agent kernel capabilities if setcap command exists
    setCap=$(command -v setcap) || setCap="/sbin/setcap" && [ -f $setCap ]  || setCap="/usr/sbin/setcap" && [ -f $setCap ] || setCap=""
    if [ -n "$setCap" ]; then
      eval "$setCap CAP_SYS_PTRACE,CAP_DAC_READ_SEARCH=+ep ${binPath}" || exit 1
    fi

    failFlag=0
    chmod 0754 "${binPath}" || failFlag=1
    if [ $failFlag -eq 1 ]; then
      # Remove capabilities given earlier if chmod fails for any reason
      eval "$setCap -r ${binPath}"
      (>&2 echo "Error setting PRIVILEGED mode. Fallbacking to UNPRIVILEGED mode")
    fi
  fi

  if [ -e "$serviceFile" ]; then
     grep 'Group=' $serviceFile >/dev/null || sed -i '/\[Service\]/aGroup=nri-agent' "$serviceFile"
     grep 'User=' $serviceFile >/dev/null || sed -i '/\[Service\]/aUser=nri-agent' "$serviceFile"
  fi
fi

if [ -e "$serviceFile" ]; then
	systemctl daemon-reload || exit $?
	systemctl enable newrelic-infra
	systemctl start newrelic-infra
fi

# Previous versions of the agent didn't remove the pid, it's removed manually
# here because current versions of the agent use a different location.
if [ -e "$oldPid" ]; then
  rm "$oldPid"
fi
