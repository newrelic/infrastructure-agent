// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux

package systemd

import (
	"os"
)

const defaultSystemBusAddress = "unix:path=/var/run/dbus/system_bus_socket"

// IsAgentRunningOnSystemD returns true if agent is running as systemd service.
func IsAgentRunningOnSystemD() bool {
	// only unix socket is supported
	_, err := os.Stat(getSystemBusPlatformAddress())
	return err == nil
}

func getSystemBusPlatformAddress() string {
	address := os.Getenv("DBUS_SYSTEM_BUS_ADDRESS")
	if address != "" {
		return address
	}
	return defaultSystemBusAddress
}
