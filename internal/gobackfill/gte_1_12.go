// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//
// Golang code for Go versions greater or equal than 1.12.
//
// +build go1.12

package gobackfill

import "os/exec"

// ExitCode wrapping to backfill old Golang versions.
// Negative values mean:
// * -1: not exited
// * -2: unknown
func ExitCode(exitError *exec.ExitError) int {
	return exitError.ExitCode()
}
