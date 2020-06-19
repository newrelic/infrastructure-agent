// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux darwin
// +build amd64

package plugins

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/agent/mocks"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/newrelic/infrastructure-agent/pkg/sample"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockPlugin struct {
	external bool
}

func (m mockPlugin) Run() {}

func (m mockPlugin) LogInfo() {}

func (m mockPlugin) ScheduleHealthCheck() {}

func (m mockPlugin) Id() ids.PluginID { return ids.PluginID{} }

func (m mockPlugin) GetExternalPluginName() string {
	return fmt.Sprintf("plugin-external-%v", m.external)
}

func (m mockPlugin) IsExternal() bool {
	return m.external
}

func TestK8sIntegrationsPlugin(t *testing.T) {
	waitC := make(chan struct{})
	ctx := new(mocks.AgentContext)
	ctx.On("Config").Return(&config.Config{
		K8sIntegrationSamplesIntervalSec: 1,
	})

	ctx.On("AgentIdentifier").Return("my-agent")
	ctx.On(
		"SendEvent",
		mock.AnythingOfType("agent.mapEvent"), entity.Key("my-agent"),
	).Return(nil).Run(func(args mock.Arguments) {
		// Retrieve Event data
		e, _ := args.Get(0).(sample.Event)
		eBytes, _ := json.Marshal(e)
		var eRaw interface{}
		err := json.Unmarshal(eBytes, &eRaw)
		if err != nil {
			t.Fatal()
		}
		eMap, _ := eRaw.(map[string]interface{})
		eType, _ := eMap["eventType"].(string)
		eIntegrationName, _ := eMap["integrationName"].(string)

		// Only the plugin with `External=true` is sent
		assert.Equal(t, "plugin-external-true", eIntegrationName)
		assert.Equal(t, "K8sIntegrationSample", eType)

		// Cannot close the channel because more ticker messages could be
		// have been triggered
		waitC <- struct{}{}
	})

	pluginRetrieveFn := func() []agent.Plugin {
		return []agent.Plugin{
			mockPlugin{false},
			mockPlugin{true},
		}
	}
	p := NewK8sIntegrationsPlugin(ctx, pluginRetrieveFn)
	ctx.On("AddReconnecting", p).Return(nil)
	pK8S, _ := p.(*K8sIntegrationsPlugin)
	pK8S.frequency = 1
	go p.Run()

	<-waitC
	ctx.AssertExpectations(t)
}
