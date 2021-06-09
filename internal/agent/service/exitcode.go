// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package service

import "fmt"

type exitCodeErr struct {
	exitCode int
}

func (e *exitCodeErr) Error() string {
	return fmt.Sprintf("returned non zero exit: %d", e.exitCode)
}

func (e *exitCodeErr) ExitCode() int {
	return e.exitCode
}

func newExitCodeErr(exitCode int) *exitCodeErr {
	return &exitCodeErr{
		exitCode: exitCode,
	}
}
