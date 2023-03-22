// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package plugins

import (
	"github.com/newrelic/infrastructure-agent/internal/agent/types"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"testing"

	"github.com/newrelic/infrastructure-agent/internal/agent/mocks"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/stretchr/testify/mock"
)

func TestConfig(t *testing.T) {
	pluginId := ids.NewPluginID("metadata", "agent_config")
	agentId := "FakeAgent"

	ctx := new(mocks.AgentContext)
	ctx.On("AddReconnecting", mock.Anything).Return()
	ctx.On("EntityKey").Return(agentId)
	ctx.On("Config").Return(config.NewConfig())
	ch := make(chan mock.Arguments)
	ctx.On("SendData", mock.Anything).Run(func(args mock.Arguments) {
		ch <- args
	})
	ctx.SendDataWg.Add(1)

	plugin := NewAgentConfigPlugin(*pluginId, ctx)
	go plugin.Run()

	args := <-ch

	_, ok := args[0].(types.PluginOutput)
	assert.True(t, ok)
	gotPluginOutput := args[0]

	expectedConf := config.NewConfig()
	expectedConfPublic, err := expectedConf.PublicFields()
	require.NoError(t, err)
	expectedInvItems := map[string]interface{}{}
	for name, value := range expectedConfPublic {
		expectedInvItems[name] = map[string]interface{}{
			"value": value,
		}
	}
	expectedPluginOutput := types.NewPluginOutput(*pluginId, entity.NewFromNameWithoutID(agentId), types.PluginInventoryDataset{ConfigAttrs(expectedInvItems)})

	assert.Equal(t, gotPluginOutput, expectedPluginOutput)
	ctx.AssertExpectations(t)

}
