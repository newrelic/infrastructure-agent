#!/bin/sh

if [ -e "/etc/init/newrelic-infra.conf" ]; then
  initctl status newrelic-infra | grep start
  RETVAL=$?
  if [ $RETVAL -eq 0 ]; then
    initctl stop newrelic-infra || exit $?
  fi
fi
