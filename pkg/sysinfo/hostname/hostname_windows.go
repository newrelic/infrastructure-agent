// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package hostname

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

func init() {
	fullHostnameResolver = getRegistryHostname
}

func getRegistryHostname(_ string) (hn string, err error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services\Tcpip\Parameters`, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer key.Close()

	domain, _, err := key.GetStringValue("Domain")
	if err != nil && err != registry.ErrNotExist {
		return "", err
	}
	if len(domain) == 0 {
		if domain, _, err = key.GetStringValue("DhcpDomain"); err != nil && err != registry.ErrNotExist {
			return "", err
		}
	}

	hostname, _, err := key.GetStringValue("Hostname")
	if err != nil && err != registry.ErrNotExist {
		return "", err
	}
	if len(hostname) == 0 {
		return "", fmt.Errorf("unknown hostname: No values found in registry. Checked Domain, DhcpDomain, and Hostname values in SYSTEM\\CurrentControlSet\\Services\\Tcpip\\Parameters")
	}

	if len(domain) == 0 {
		return "", fmt.Errorf("unknown domain name")
	}
	return fmt.Sprintf("%s.%s", hostname, domain), nil
}

// If we invoke this function from Windows, that means that the "getRegistryHostname" didn't work properly, so we try
// returning the short hostname
func internalHostname() (string, error) {
	return os.Hostname()
}
