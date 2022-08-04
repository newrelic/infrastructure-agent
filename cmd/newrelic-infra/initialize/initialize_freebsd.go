// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// Package initialize performs OS-specific initialization actions during the
// startup of the agent. The execution order of the functions in this package is:
// 1 - OsProcess (when the operating system process starts and the configuration is loaded)
// 2 - AgentService (before the Agent starts)
package initialize

import (
	"github.com/newrelic/infrastructure-agent/pkg/config"
)

// AgentService performs OS-specific initialization steps for the Agent service.
// It is executed after the initialize.osProcess function.
func AgentService(cfg *config.Config) error {
	return nil
}

// OsProcess performs initialization steps for the OS process that contains the
// agent. It is executed before the initialize.AgentService function.
func OsProcess(config *config.Config) error {
	return nil
}
