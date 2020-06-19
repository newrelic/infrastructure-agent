// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package hostname

import "github.com/newrelic/infrastructure-agent/pkg/helpers"

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
