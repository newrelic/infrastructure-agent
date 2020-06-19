#!/bin/sh

if [ -e "/etc/systemd/system/newrelic-infra.service" ]; then
  systemctl disable newrelic-infra
  systemctl status newrelic-infra | grep "active (running)"
  RETVAL=$?
  if [ $RETVAL -eq 0 ]; then
    systemctl stop newrelic-infra || exit $?
  fi
fi
