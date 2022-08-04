// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux || darwin || freebsd
// +build linux darwin freebsd

package signals

import (
	"syscall"
)

const (
	// Notification signal is used to send notification. Used for Linux ctl verbose mode notifier.
	Notification = syscall.SIGUSR1
	// GracefulStop signal is used to gracefully stop, we use SIGTSTP as SIGSTOP can not be handled.
	GracefulStop = syscall.SIGUSR2
)
