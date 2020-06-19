// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package plugins

import (
	"reflect"

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

func (self ConfigAttrs) SortKey() string {
	return "infrastructure"
}

func NewAgentConfigPlugin(id ids.PluginID, ctx agent.AgentContext) agent.Plugin {
	return &AgentConfigPlugin{
		agent.PluginCommon{ID: id, Context: ctx},
		*ctx.Config(),
	}
}

// This plugin is pretty simple - it simply returns once with the object containing the agent's config settings
func (self *AgentConfigPlugin) Run() {
	self.Context.AddReconnecting(self)

	self.config.License = ""
	if self.config.Proxy != "" {
		self.config.Proxy = "<proxy set>"
	}

	flat := map[string]interface{}{}
	value := reflect.ValueOf(self.config)
	for i := 0; i < value.NumField(); i++ {
		name := value.Type().Field(i).Name
		switch name {
		case "FilesConfigOn", "DebugLogSec", "OfflineLoggingMode":
			continue
		default:
			if value.Field(i).CanInterface() {
				fieldValue := value.Field(i).Interface()
				flat[name] = map[string]interface{}{
					"value": fieldValue,
				}
			}
		}
	}

	helpers.LogStructureDetails(aclog, flat, "config", "raw", logrus.Fields{})

	self.EmitInventory(agent.PluginInventoryDataset{ConfigAttrs(flat)}, self.Context.AgentIdentifier())
}
