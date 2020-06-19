// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package signals

const (
	// NotificationStr string representation for signal used to send notification. Used for Docker.
	NotificationStr = "SIGUSR1"
	GracefulStopStr = "SIGUSR2"
	// GracefulShutdownStr is not a real POSIX signal, it's a custom signal we use when we detect a host shutdown
	GracefulShutdownStr = "SHUTDOWN"
)
