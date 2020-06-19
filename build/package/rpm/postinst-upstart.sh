#!/bin/sh

serviceFile=/etc/init/newrelic-infra.conf
oldPid=/var/run/newrelic-infra.pid
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
  chown -R nri-agent:nri-agent ${configDir}
  chown -R nri-agent:nri-agent ${logDir}
  chown -R nri-agent:nri-agent ${installDir}
  chown -R nri-agent:nri-agent ${tmpDir} 2>/dev/null || true

  if [ "$userMode" = "PRIVILEGED" ]; then
    # Give the Agent kernel capabilities if setcap command exists
    setCap=$(command -v setcap) || setCap="/sbin/setcap" && [ -f $setCap ] || setCap=""
    if [ ! -z $setCap ]; then
      eval "$setCap CAP_SYS_PTRACE,CAP_DAC_READ_SEARCH=+ep /usr/bin/newrelic-infra" || exit 1
    fi
  fi

  if [ -e "$serviceFile" ]; then
    sed -i '/chdir \/root\//d' "$serviceFile"
    # Upstart in CentOS 6 is very old and doesn't provide support for setuid or setgid
    sed -i "s#exec /usr/bin/newrelic-infra#exec su -s /bin/sh -c 'exec \"\$0\" \"\$@\"' nri-agent -- /usr/bin/newrelic-infra#g" "$serviceFile"
    # Set permissions to the /var/run/newrelic-infra folder
    sed -i 's/#permissions/chown -R nri-agent:nri-agent \/var\/run\/newrelic-infra/g' "$serviceFile"
  fi

else
  if [ -e "$serviceFile" ]; then
     # Set correct values when running as root
    grep '^chdir' "$serviceFile" || sed -i '/set working directory/achdir \/root\/' "$serviceFile"
    sed -i "s#exec su -s /bin/sh -c 'exec \"\$0\" \"\$@\"' nri-agent -- /usr/bin/newrelic-infra#exec /usr/bin/newrelic-infra#g" "$serviceFile"
  fi
fi

if [ -e "$serviceFile" ]; then
    initctl start newrelic-infra || exit $?
fi

# Previous versions of the agent didn't remove the pid, it's removed manually
# here because current versions of the agent use a different location.
if [ -e "$oldPid" ]; then
    rm "$oldPid"
fi
