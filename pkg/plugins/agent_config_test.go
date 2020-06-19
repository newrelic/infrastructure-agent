// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package plugins

import (
	"github.com/stretchr/testify/assert"
	"testing"

	"reflect"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/agent/mocks"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/stretchr/testify/mock"
)

func configAsMap(config *config.Config) map[string]interface{} {
	out := map[string]interface{}{}

	value := reflect.ValueOf(*config)
	for i := 0; i < value.NumField(); i++ {
		name := value.Type().Field(i).Name
		if value.Field(i).CanInterface() {
			fieldValue := value.Field(i).Interface()
			out[name] = map[string]interface{}{
				"value": fieldValue,
			}
		}
	}

	return out
}

func getConfigAsInventoryDataset(config config.Config) agent.PluginInventoryDataset {
	// These values are private
	config.License = ""
	if config.Proxy != "" {
		config.Proxy = "<proxy set>"
	}
	configMap := configAsMap(&config)

	// These config options are not sent
	delete(configMap, "FilesConfigOn")
	delete(configMap, "DebugLogSec")
	delete(configMap, "OfflineLoggingMode")

	return agent.PluginInventoryDataset{ConfigAttrs(configMap)}
}

func TestConfig(t *testing.T) {
	pluginId := ids.NewPluginID("metadata", "agent_config")
	agentId := "FakeAgent"

	ctx := new(mocks.AgentContext)
	ctx.On("AddReconnecting", mock.Anything).Return()
	ctx.On("AgentIdentifier").Return(agentId)
	ctx.On("Config").Return(config.NewConfig())
	ch := make(chan mock.Arguments)
	ctx.On("SendData", mock.Anything).Run(func(args mock.Arguments) {
		ch <- args
	})

	plugin := NewAgentConfigPlugin(*pluginId, ctx)
	go plugin.Run()

	args := <-ch

	_, ok := args[0].(agent.PluginOutput)
	assert.True(t, ok)
	expectedPluginOutput := args[0]

	configInventory := getConfigAsInventoryDataset(*config.NewConfig())
	actualPluginOutput := agent.NewPluginOutput(*pluginId, agentId, configInventory)

	assert.Equal(t, expectedPluginOutput, actualPluginOutput)
	ctx.AssertExpectations(t)

}
