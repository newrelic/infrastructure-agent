// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//
// Golang code for Go versions lower than 1.12.
//
// +build !go1.12

package gobackfill

import "os/exec"

// ExitCode not supported, always value -2, as -1 belong to not finished.
func ExitCode(exitError *exec.ExitError) int {
	return -2
}
