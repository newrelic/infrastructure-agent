// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux

package v4

import (
	"path/filepath"

	"github.com/newrelic/infrastructure-agent/pkg/helpers"
)

const (
	// defaults for td-agent-bit (<=1.9).
	defaultLoggingBinDir1       = "/opt/td-agent-bit/bin"
	defaultFluentBitExecutable1 = "td-agent-bit"
	// defaults for fluent-bit (>=2.0).
	defaultLoggingBinDir2       = "/opt/fluent-bit/bin"
	defaultFluentBitExecutable2 = "fluent-bit"
)

func (c *fBSupervisorConfig) defaultLoggingBinDir(_ bool, _ bool) string {
	if onlyTdAgentInstalled() {
		return defaultLoggingBinDir1
	}

	return defaultLoggingBinDir2
}

func (c *fBSupervisorConfig) defaultFluentBitExePath(_ bool, _ bool, loggingBinDir string) string {
	defaultFluentBitExe := defaultFluentBitExecutable2
	if onlyTdAgentInstalled() {
		defaultFluentBitExe = defaultFluentBitExecutable1
	}

	return filepath.Join(loggingBinDir, defaultFluentBitExe)
}

func onlyTdAgentInstalled() bool {
	fbExePath := filepath.Join(defaultLoggingBinDir2, defaultFluentBitExecutable2)
	tdExePath := filepath.Join(defaultLoggingBinDir1, defaultFluentBitExecutable1)

	return helpers.FileExists(tdExePath) && !helpers.FileExists(fbExePath)
}
