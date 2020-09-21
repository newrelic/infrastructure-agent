// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package integration

import (
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/executor"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
)

// DefinitionCommandConfig provides curated configuration to create a DefinitionCommand instance.
// It is created from the Definition and Command YAML structs
type DefinitionCommandConfig struct {
	Common          Definition
	IntegrationName string
	Command         string
	Arguments       map[string]string
	// If no 'prefix' is defined, it will inherit the default (or used-defined) inventory_source
	DefaultPrefix ids.PluginID
}

// FromLegacyDefinition allows acquiring building up an integration.Definition that refers to
// an entry in any of the existing /var/db/newrelic-infra/*-integrations/*-definition.yml legacy file.
// Those definitions are considered legacy, but they are left here for backwards-compatibility
// purposes with our OHIs
func FromLegacyDefinition(cfg DefinitionCommandConfig, runnable executor.Executor) Definition {
	ilog.
		WithField("integration_name", cfg.Common.Name).
		Debug("Wrapping v3 definition command into a v4 executable.")
	// For simplicity, we wrap the v3 definition command into a v4 executable
	d := cfg.Common
	d.runnable = runnable
	d.newTempFile = newTempFile
	return d
}
