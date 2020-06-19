#!/bin/sh

if [ -e "/etc/init/newrelic-infra.conf" ] || [ -e "/etc/init.d/newrelic-infra" ]; then
	invoke-rc.d newrelic-infra status
	RETVAL=$?
	if [ $RETVAL -eq 0 ]; then
		invoke-rc.d newrelic-infra stop || exit $?
	fi
fi
