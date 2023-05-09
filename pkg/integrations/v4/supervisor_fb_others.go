//go:build !windows
// +build !windows

/*
 * Copyright 2021 New Relic Corporation. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package v4

func addOSDependantArgs(args []string) []string {
	return args
}
