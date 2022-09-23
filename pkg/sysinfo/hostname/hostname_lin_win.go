// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux || windows
// +build linux windows

package hostname

import (
	"errors"
	"net"
	"strings"

	"github.com/newrelic/infrastructure-agent/pkg/config"
)

// Looks up for the Fully Qualified Domain Name.
// `localhost` should not be returned as FQDN, but until deciding how it affects we will maintain this version
// for linux and windows for backwards compatibility
func getFqdnHostname(osHost string) (string, error) {
	ips, err := net.LookupIP(osHost)
	if err != nil {
		return "", err
	}

	for _, ip := range ips {
		hosts, err := net.LookupAddr(ip.String())
		if err != nil || len(hosts) == 0 {
			return "", err
		}
		logger.
			WithField(config.TracesFieldName, config.FeatureTrace).
			Tracef("found FQDN hosts: %s", strings.Join(hosts, ", "))
		return strings.TrimSuffix(hosts[0], "."), nil
	}
	return "", errors.New("can't lookup FQDN")
}
