// Copyright 2024 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package plugins

import (
	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/agent/types"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
)

// IntegrationsOnlyPlugin is an inventory plugin that will add an inventory entry notifying the agent is running on
// this mode. This inventory entry will be read by Infra Platform to not send the Host Entity to Entity Platform.
type IntegrationsOnlyPlugin struct {
	agent.PluginCommon
}

type IntegrationsOnly bool

func (io IntegrationsOnly) SortKey() string {
	return "integrations_only"
}

func NewIntegrationsOnlyPlugin(id ids.PluginID, ctx agent.AgentContext) agent.Plugin { //nolint:ireturn
	return &IntegrationsOnlyPlugin{
		//nolint:exhaustruct
		PluginCommon: agent.PluginCommon{
			ID:      id,
			Context: ctx,
		},
	}
}

// Run This plugin is pretty simple - it simply returns once with the object notifying that integrations_only is enabled.
func (iop *IntegrationsOnlyPlugin) Run() {
	iop.Context.AddReconnecting(iop)

	data := types.PluginInventoryDataset{IntegrationsOnly(true)}
	entityKey := iop.Context.EntityKey()

	iop.EmitInventory(data, entity.NewFromNameWithoutID(entityKey))
}
