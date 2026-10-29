// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build windows

package v4

import "path/filepath"

const (
	// defaults for fluent-bit (>=2.0). The legacy td-agent-bit (<=1.9) is no longer bundled.
	defaultLoggingBinDir = "logging"
	defaultFluentBitExe  = "fluent-bit.exe"
)

func (c *fBSupervisorConfig) defaultLoggingBinDir(_ bool, _ bool) string {
	return filepath.Join(c.agentDir, c.integrationsDir, defaultLoggingBinDir)
}

func (c *fBSupervisorConfig) defaultFluentBitExePath(_ bool, _ bool, loggingBinDir string) string {
	return filepath.Join(loggingBinDir, defaultFluentBitExe)
}
