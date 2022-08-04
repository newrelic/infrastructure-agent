// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build freebsd
// +build freebsd

package hostname

import (
	"errors"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"net"
	"strings"
)

// attempts to determine the hostname, gracefully falling back until we
// run out of options
func internalHostname() (hn string, err error) {
	// attempt to fetch FQDN
	hn, err = helpers.RunCommand("/usr/bin/env", "", "hostname", "-f")
	if err == nil && hn != "" {
		return
	}

	// failing that try the short name
	hn, err = helpers.RunCommand("/usr/bin/env", "", "hostname")
	if err == nil && hn != "" {
		return
	}

	// return whatever we did get including the error
	return
}

type ipLookupper func(host string) ([]net.IP, error)
type addrLookupper func(addr string) (names []string, err error)

var lookupIp ipLookupper = net.LookupIP
var lookupAddr addrLookupper = net.LookupAddr

// Looks up for the Fully Qualified Domain Name. Do not take into account localhost as FQDN
func getFqdnHostname(osHost string) (string, error) {
	ips, err := lookupIp(osHost)
	if err != nil {
		return "", err
	}

	for _, ip := range ips {
		hosts, err := lookupAddr(ip.String())
		if err != nil || len(hosts) == 0 {
			return "", err
		}
		if hosts[0] == "localhost" {
			continue
		}
		logger.Tracef("found FQDN hosts: %s", strings.Join(hosts, ", "))
		return strings.TrimSuffix(hosts[0], "."), nil
	}
	return "", errors.New("can't lookup FQDN")
}
