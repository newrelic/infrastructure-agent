// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build windows
// +build windows

package ipc

const (
	EnableVerboseLogging Message = "notification"
	Stop                 Message = "stop"
	Shutdown             Message = "shutdown"
)
