// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build windows

package v4

import "path/filepath"

const (
	// defaults for td-agent-bit (<=1.9).
	defaultLoggingBinDir1 = "logging"
	// defaults for fluent-bit (>=2.0).
	defaultLoggingBinDir2 = "logging"
	// both versions have the same name.
	defaultFluentBitExe = "fluent-bit.exe"
)

func (c *fBSupervisorConfig) defaultLoggingBinDir(ffExists bool, ffEnabled bool) string {
	loggingBinDir := defaultLoggingBinDir2
	if ffExists && ffEnabled {
		loggingBinDir = defaultLoggingBinDir1
	}
	return filepath.Join(c.agentDir, c.integrationsDir, loggingBinDir)
}

func (c *fBSupervisorConfig) defaultFluentBitExePath(_ bool, _ bool, loggingBinDir string) string {
	return filepath.Join(loggingBinDir, defaultFluentBitExe)
}
