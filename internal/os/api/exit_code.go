// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package api

import (
	"os"
	"os/exec"
	"syscall"
)

const (
	// ExitCodeSuccess is returned when the process has finished successfully.
	ExitCodeSuccess = 0
	// ExitCodeError is returned when the process has failed.
	ExitCodeError = 1
	// ExitCodeRestart is a request from the process to be restarted.
	ExitCodeRestart = 3
)

// CheckExitCode checks the error for the status code.
func CheckExitCode(err error) int {
	if err == nil {
		return ExitCodeSuccess
	}

	exitErr, isExitError := err.(*exec.ExitError)
	if !isExitError {
		os.Exit(ExitCodeError)
	}

	if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
		return status.ExitStatus()
	}

	return ExitCodeError
}
