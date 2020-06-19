// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package network_helpers

import (
	"strings"

	"github.com/shirou/gopsutil/net"
)

const (
	IPV6_MARKER = ":"
)

type InterfacesProvider func() ([]net.InterfaceStat, error)

func GopsutilInterfacesProvider() ([]net.InterfaceStat, error) {
	return net.Interfaces()
}

// Returns true if we should completely ignore the given interface.
func ShouldIgnoreInterface(filters map[string][]string, name string) bool {
	if filters != nil && len(filters) > 0 {
		ln := strings.ToLower(name)
		for op, nica := range filters {
			for _, nic := range nica {
				lNicTest := strings.ToLower(nic)
				switch op {
				case "prefix":
					if strings.HasPrefix(ln, lNicTest) {
						return true
					}
				case "index-1":
					if strings.Index(ln, lNicTest) == 1 {
						return true
					}
				}
			}
		}
	}
	return false
}

// Returns if the provided string represents a valid IPv6 address.
func IsAddressIPv6(s string) bool {
	return strings.ContainsAny(s, IPV6_MARKER)
}

// Returns the IPv4 & IPv6 addresses from the list of InterfaceAddr. If
// multiple addresses of the same type exist, the last seen takes precedence.
func IPAddressesByType(addrs []net.InterfaceAddr) (ipv4, ipv6 string) {
	for _, ia := range addrs {
		if IsAddressIPv6(ia.Addr) {
			ipv6 = ia.Addr
		} else {
			ipv4 = ia.Addr
		}
	}
	return
}
