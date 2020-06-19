#!/bin/sh
/usr/sbin/squid -NYCd 1 &
sleep 3
tail -f /var/log/squid/access.log
