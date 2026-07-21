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
	"path/filepath"

	"github.com/newrelic/infrastructure-agent/pkg/config"
	v4 "github.com/newrelic/infrastructure-agent/pkg/integrations/v4"
)

const tempFolderMode = 0o755

var (
	removeFunc = os.RemoveAll // nolint:gochecknoglobals
	mkdirFunc  = os.MkdirAll  // nolint:gochecknoglobals
)

// emptyFbConfigTempFolder deletes all files inside the FluentBit config temp folder
// (<AgentTempDir>/fb), regardless of whether AgentTempDir is set to its default value
// or to a custom location via NRIA_AGENT_TEMP_DIR. It's scoped to the "fb" subfolder,
// not the whole AgentTempDir, since the latter may point at a shared system temp
// directory (e.g. the Linux/macOS default is os.TempDir()).
func emptyFbConfigTempFolder(cfg *config.Config) error {
	if cfg.AgentTempDir == "" {
		return nil
	}

	fbTempFolder := filepath.Join(cfg.AgentTempDir, v4.FbConfTempFolderNameDefault)

	err := removeFunc(fbTempFolder)
	if err != nil {
		return fmt.Errorf("can't empty agent temporary folder: %w", err) //nolint:wrapcheck
	}

	err = mkdirFunc(fbTempFolder, tempFolderMode)
	if err != nil {
		return fmt.Errorf("can't create agent temporary folder: %w", err) //nolint:wrapcheck
	}

	return nil
}
