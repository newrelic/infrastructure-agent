// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
)

type dummyPlugin struct {
	agent.PluginCommon
	ticker chan interface{}
	value  string
}

type valueEntry struct {
	Id    string `json:"id"`
	Value string `json:"value"`
}

func (v *valueEntry) SortKey() string {
	return v.Id
}

func newDummyPlugin(initialValue string, context agent.AgentContext) *dummyPlugin {
	return &dummyPlugin{
		PluginCommon: agent.PluginCommon{
			ID:      ids.PluginID{"test", "dummy"},
			Context: context,
		},
		ticker: make(chan interface{}),
		value:  initialValue,
	}
}

func (cp *dummyPlugin) Run() {
	for {
		select {
		case <-cp.ticker:
			dataset := agent.PluginInventoryDataset{
				&valueEntry{Id: "dummy", Value: cp.value},
			}

			var eKey string
			if cp.IsExternal() {
				eKey = cp.ExternalPluginName
			} else {
				eKey = cp.Context.AgentIdentifier()
			}
			cp.EmitInventory(dataset, entity.NewFromNameWithoutID(eKey))
		}
	}
}

func (cp *dummyPlugin) Id() ids.PluginID {
	return cp.ID
}

func (cp *dummyPlugin) harvest() {
	cp.ticker <- 1
}
