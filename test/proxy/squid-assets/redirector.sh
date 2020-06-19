#!/bin/bash

# This redirector does not do any redirection. It is only used as a "hook" to notify to the collector that this proxy
# is being used
# We do this way because you can't transparently modify the URL, Body or Headers of an HTTPS request
while read s; do
    curl -k -X POST -d 'http-proxy' https://fake-collector:4444/notifyproxy
    echo 'OK'
done
