// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package plugins

import (
	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
)

func isHeartbeatOnlyMode(cfg *config.Config) bool {
	return cfg.IsSecureForwardOnly || cfg.IsIntegrationsOnly
}

func registerIntegrationsOnlyPlugin(agt *agent.Agent) {
	cfg := agt.GetContext().Config()
	if cfg.IsSecureForwardOnly {
		slog.Debug("Both Integrations Only mode and Secure Forward Only are enabled, skipping integrations only mode")

		return
	}

	agt.RegisterPlugin(NewIntegrationsOnlyPlugin(ids.PluginID{Category: "metadata", Term: "infra_agent"}, agt.Context))
}
