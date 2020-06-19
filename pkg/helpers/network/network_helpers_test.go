// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package network_helpers

import (
	"github.com/shirou/gopsutil/net"

	. "gopkg.in/check.v1"
)

type NetworkHelpersSuite struct{}

var _ = Suite(&NetworkHelpersSuite{})

func (s *NetworkHelpersSuite) TestShouldIgnoreInterface(c *C) {
	testInterfaces := []struct {
		name   string
		result bool
	}{
		{"lo", true},
		{"twooffset", false},
		{"in-middle0", false},
		{"xface", true},
	}

	filters := map[string][]string{
		"prefix":  {"lo", "offset"},
		"index-1": {"middle", "face"},
	}

	for _, ti := range testInterfaces {
		c.Assert(ShouldIgnoreInterface(filters, ti.name), Equals, ti.result)
	}

}

func (s *NetworkHelpersSuite) TestIsAddressIPv6(c *C) {
	testCases := []struct {
		address string
		result  bool
	}{
		{"192.168.0.1", false},
		{"127.0.0.1", false},
		{"0.0.0.0", false},
		{"2001:0db8:85a3:0000:0000:8a2e:0370:7334", true},
		{"2001:db8:85a3:0:0:8a2e:370:7334", true},
		{"2001:db8:85a3::8a2e:370:7334", true},
		{"0:0:0:0:0:0:0:0", true},
		{"::", true},
		{"0:0:0:0:0:0:0:1", true},
		{"::1", true},
		{"::ffff:192.0.2.128", true}, // IPv4-mapped IPv6 address
	}
	for _, tc := range testCases {
		c.Assert(IsAddressIPv6(tc.address), Equals, tc.result)
	}
}

func (s *NetworkHelpersSuite) TestIPAddressesByType(c *C) {
	ipv4, ipv6 := IPAddressesByType([]net.InterfaceAddr{
		{"192.168.0.1"},
		{"127.0.0.1"},
		{"::1"},
		{"2001:db8:85a3::8a2e:370:7334"},
	})
	c.Assert(ipv4, Equals, "127.0.0.1")
	c.Assert(ipv6, Equals, "2001:db8:85a3::8a2e:370:7334")

	ipv4, ipv6 = IPAddressesByType(nil)
	c.Assert(ipv4, Equals, "")
	c.Assert(ipv6, Equals, "")
}
