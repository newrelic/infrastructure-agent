// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package api

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/newrelic/infrastructure-agent/pkg/log"
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
		log.WithError(err).Error("error occurred while running the agent process, exiting...")
		os.Exit(ExitCodeError)
	}

	if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
		return status.ExitStatus()
	}

	return ExitCodeError
}

// ExitCodeErr error representing a CLI exit status code.
type ExitCodeErr struct {
	exitCode int
}

func (e *ExitCodeErr) Error() string {
	return fmt.Sprintf("returned non zero exit: %d", e.exitCode)
}

func (e *ExitCodeErr) ExitCode() int {
	return e.exitCode
}

func NewExitCodeErr(exitCode int) *ExitCodeErr {
	return &ExitCodeErr{
		exitCode: exitCode,
	}
}
