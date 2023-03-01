// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// Package initialize performs OS-specific initialization actions during the
// startup of the agent. The execution order of the functions in this package is:
// 1 - OsProcess (when the operating system process starts and the configuration is loaded)
// 2 - AgentService (before the Agent starts)
package initialize

import (
	"fmt"
	"os"

	"github.com/newrelic/infrastructure-agent/pkg/config"
)

const tempFolderMode = 0o755

var (
	removeFunc = os.RemoveAll // nolint:gochecknoglobals
	mkdirFunc  = os.MkdirAll  // nolint:gochecknoglobals
)

// nolint:godot
// emptyTemporaryFolder deletes all files inside the default agent's temporary folder,
// only if configuration option matches the default value.
//
// Default (Linux): /var/db/newrelic-infra/tmp
// Default (MacOS AMD): /usr/local/var/db/newrelic-infra/tmp
// Default (MacOS ARM): /opt/homebrew/var/db/newrelic-infra/tmp
// Default (Windows): c:\ProgramData\New Relic\newrelic-infra\tmp
func emptyTemporaryFolder(cfg *config.Config) error {
	if cfg.AgentTempDir == agentTemporaryFolder {
		err := removeFunc(agentTemporaryFolder)
		if err != nil {
			return fmt.Errorf("can't empty agent temporary folder: %w", err)
		}

		err = mkdirFunc(agentTemporaryFolder, tempFolderMode)
		if err != nil {
			return fmt.Errorf("can't create agent temporary folder: %w", err)
		}
	}

	return nil
}
