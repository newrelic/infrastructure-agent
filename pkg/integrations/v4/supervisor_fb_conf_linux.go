// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux

package v4

import "path/filepath"

const (
	// defaults for td-agent-bit (<=1.9).
	defaultLoggingBinDir1 = "/opt/td-agent-bit/bin"
	defaultFluentBitExe1  = "td-agent-bit"
	// defaults for fluent-bit (>=2.0).
	defaultLoggingBinDir2 = "/opt/fluent-bit/bin"
	defaultFluentBitExe2  = "fluent-bit"
)

func (c *fBSupervisorConfig) defaultLoggingBinDir(ffExists bool, ffEnabled bool) string {
	if ffExists && ffEnabled {
		return defaultLoggingBinDir1
	}

	return defaultLoggingBinDir2
}

func (c *fBSupervisorConfig) defaultFluentBitExePath(ffExists bool, ffEnabled bool, loggingBinDir string) string {
	defaultFluentBitExe := defaultFluentBitExe2
	if ffExists && ffEnabled {
		defaultFluentBitExe = defaultFluentBitExe1
	}

	return filepath.Join(loggingBinDir, defaultFluentBitExe)
}
