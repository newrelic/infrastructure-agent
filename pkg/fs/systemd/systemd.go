// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build !linux

package systemd

// IsAgentRunningOnSystemD returns true if agent is running as systemd service.
func IsAgentRunningOnSystemD() bool {
	return false
}
