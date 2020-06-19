// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package testhelpers

import (
	"net"
)

// GetFreeTCPPort returns a free open TCP port
func GetFreeTCPPort() (port int, err error) {
	ln, err := net.Listen("tcp", "[::]:0")
	if err != nil {
		return 0, err
	}
	port = ln.Addr().(*net.TCPAddr).Port
	err = ln.Close()
	return
}
