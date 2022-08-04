// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux || darwin || freebsd
// +build linux darwin freebsd

package ipc

import (
	"github.com/newrelic/infrastructure-agent/internal/os/api/signals"
)

const (
	EnableVerboseLogging Message = signals.NotificationStr
	Stop                 Message = signals.GracefulStopStr
	Shutdown             Message = signals.GracefulShutdownStr
)
