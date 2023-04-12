// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package plugins

import (
	"github.com/newrelic/infrastructure-agent/internal/agent/types"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/sirupsen/logrus"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
)

var aclog = log.WithPlugin("AgentConfig")

type AgentConfigPlugin struct {
	agent.PluginCommon
	config config.Config
}

type ConfigAttrs map[string]interface{}

func (ac ConfigAttrs) SortKey() string {
	return "infrastructure"
}

func NewAgentConfigPlugin(id ids.PluginID, ctx agent.AgentContext) agent.Plugin {
	return &AgentConfigPlugin{
		PluginCommon: agent.PluginCommon{ID: id, Context: ctx},
		config:       *ctx.Config(),
	}
}

// This plugin is pretty simple - it simply returns once with the object containing the agent's config settings
func (ac *AgentConfigPlugin) Run() {
	ac.Context.AddReconnecting(ac)

	ac.config.License = ""
	if ac.config.Proxy != "" {
		ac.config.Proxy = "<proxy set>"
	}

	fields, err := ac.config.PublicFields()
	if err != nil {
		aclog.WithError(err).Error("cannot retrieve public config fields")
		return
	}

	inventoryItems := map[string]interface{}{}
	for name, value := range fields {
		inventoryItems[name] = map[string]interface{}{
			"value": value,
		}
	}

	helpers.LogStructureDetails(aclog, inventoryItems, "config", "raw", logrus.Fields{})

	ac.EmitInventory(types.PluginInventoryDataset{ConfigAttrs(inventoryItems)}, entity.NewFromNameWithoutID(ac.Context.EntityKey()))
}
