// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package config

import (
	"os"
	"path/filepath"
)

func init() { //nolint:gochecknoinits
	defaultConfigFiles = []string{
		"newrelic-infra.yml",
		filepath.Join("/opt", "homebrew", "etc", "newrelic-infra", "newrelic-infra.yml"),
	}
	defaultAgentDir = filepath.Join("/opt", "homebrew", "var", "db", "newrelic-infra")
	defaultSafeBinDir = defaultAgentDir
	defaultAgentTempDir = os.TempDir()
}
