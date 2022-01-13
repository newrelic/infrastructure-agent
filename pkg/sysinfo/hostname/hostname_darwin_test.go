// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build darwin
// +build darwin

package hostname

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
)

func ipLookUpperMock(ips []net.IP, err error) func(host string) ([]net.IP, error) {
	return func(_ string) ([]net.IP, error) {
		return ips, err
	}
}

func addrLookUpperMock(names map[string][]string, err error) func(addr string) (names []string, err error) {
	return func(lookUpIp string) ([]string, error) {
		return names[lookUpIp], err
	}
}

func TestGetFqdnHostname_DontReturnLocalhost(t *testing.T) {

	tests := []struct {
		name             string
		ips              []net.IP
		fqdns            map[string][]string
		ipLookupError    error
		addrLookupError  error
		expectedHostname string
		expectedErr      error
	}{
		{
			name:             "no ips should return error",
			expectedErr:      errors.New("can't lookup FQDN"),
			expectedHostname: "",
		},
		{
			name:             "only localhost should return error",
			ips:              []net.IP{net.ParseIP("127.0.0.1")},
			fqdns:            map[string][]string{"127.0.0.1": {"localhost"}},
			expectedErr:      errors.New("can't lookup FQDN"),
			expectedHostname: "",
		},
		{
			name: "first valid fqdn should be returned",
			ips:  []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("192.168.1.1"), net.ParseIP("10.0.0.1")},
			fqdns: map[string][]string{
				"127.0.0.1":   {"localhost"},
				"192.168.1.1": {"test.local"},
				"10.0.0.1":    {"another.test.local"},
			},
			expectedHostname: "test.local",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookupIp = ipLookUpperMock(tt.ips, tt.ipLookupError)
			lookupAddr = addrLookUpperMock(tt.fqdns, tt.addrLookupError)

			hostname, err := getFqdnHostname("some")

			assert.Equal(t, tt.expectedErr, err)
			assert.Equal(t, tt.expectedHostname, hostname)
		})
	}
}
