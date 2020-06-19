// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package recover

import (
	"runtime/debug"

	log "github.com/sirupsen/logrus"
)

// Type will determine the behaviour of the PanicHandler.
type Type int

const (
	// LogAndFail will cause the main process to exit after the error information is logged.
	LogAndFail Type = iota
	// LogAndContinue will log a message and after that the main process will continue.
	LogAndContinue
)

// PanicHandler can be used to capture the stack trace and print it to logs.
// It will capture panics from its running go routine.
func PanicHandler(recoverType Type) {
	r := recover()

	if r == nil {
		return
	}

	logEntry := log.WithField("stacktrace", string(debug.Stack()))

	if recoverType == LogAndFail {
		logEntry.Fatal(r)
	}
	logEntry.Error(r)
}

// FuncWithPanicHandler is a wrapper with PanicHandler for a function that is supposed to be run in a separate goroutine.
func FuncWithPanicHandler(recoverType Type, function func()) {
	defer PanicHandler(recoverType)

	function()
}
