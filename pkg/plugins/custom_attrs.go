// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package plugins

import (
	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/agent/types"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
)

type CustomAttrsPlugin struct {
	agent.PluginCommon
	customAttributes map[string]interface{}
}

type CustomAttrs map[string]interface{}

func (self CustomAttrs) SortKey() string {
	return "customAttributes"
}

func NewCustomAttrsPlugin(ctx agent.AgentContext) agent.Plugin {
	return &CustomAttrsPlugin{
		PluginCommon: agent.PluginCommon{
			ID:      ids.CustomAttrsID,
			Context: ctx,
		},
		customAttributes: ctx.Config().CustomAttributes,
	}
}

// This plugin is pretty simple - it simply returns once with the object containing current custom attributes.
func (self *CustomAttrsPlugin) Run() {
	self.Context.AddReconnecting(self)

	data := types.PluginInventoryDataset{CustomAttrs(self.customAttributes)}
	entityKey := self.Context.EntityKey()

	aclog.
		WithField(config.TracesFieldName, config.FeatureTrace).
		Tracef("run, entity: %s, data: %+v", entityKey, self.customAttributes)

	self.EmitInventory(data, entity.NewFromNameWithoutID(entityKey))
}
