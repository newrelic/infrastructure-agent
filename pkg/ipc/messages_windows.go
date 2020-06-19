// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build windows

package ipc

const (
	EnableVerboseLogging Message = "notification"
	Stop                 Message = "stop"
	Shutdown             Message = "shutdown"
)
