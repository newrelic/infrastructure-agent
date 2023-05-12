// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build !linux && !windows

package v4

func (c *fBSupervisorConfig) defaultLoggingBinDir(_ bool, _ bool) string {
	return ""
}

func (c *fBSupervisorConfig) defaultFluentBitExePath(_ bool, _ bool, _ string) string {
	return ""
}
