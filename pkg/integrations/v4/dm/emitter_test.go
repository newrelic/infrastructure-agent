// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package dm

import (
	"fmt"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/executor"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/sirupsen/logrus"
	logTest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"sync"
	"testing"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/handler"
	"github.com/newrelic/infrastructure-agent/internal/agent/mocks"
	"github.com/newrelic/infrastructure-agent/internal/feature_flags"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi/test"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/entity/host"
	"github.com/newrelic/infrastructure-agent/pkg/fwrequest"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo"
	integrationFixture "github.com/newrelic/infrastructure-agent/test/fixture/integration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var (
	testIdentity = entity.Identity{
		ID:   1,
		GUID: "abcdef",
	}
)

func TestParsePayloadV4(t *testing.T) {
	ffm := feature_flags.NewManager(map[string]bool{handler.FlagProtocolV4: true})

	d, err := ParsePayloadV4(integrationFixture.ProtocolV4.Payload, ffm)
	assert.NoError(t, err)
	assert.EqualValues(t, integrationFixture.ProtocolV4.ParsedV4, d)
}

func TestParsePayloadV4_noFF(t *testing.T) {
	ffm := feature_flags.NewManager(map[string]bool{})

	_, err := ParsePayloadV4(integrationFixture.ProtocolV4.Payload, ffm)
	assert.Equal(t, ProtocolV4NotEnabledErr, err)
}

type mockedMetricsSender struct {
	mock.Mock
	wg sync.WaitGroup
}

func (m *mockedMetricsSender) SendMetrics(metrics []protocol.Metric) {
	m.Called(metrics)
	m.wg.Done()
}

func (m *mockedMetricsSender) SendMetricsWithCommonAttributes(commonAttributes protocol.Common, metrics []protocol.Metric) error {
	err := m.Called(commonAttributes, metrics).Error(0)
	m.wg.Done()
	return err
}

func TestEmitter_Send_usingIDCache(t *testing.T) {
	eID := entity.ID(1234)
	aCtx := getAgentContext("bob")
	aCtx.On("SendData",
		agent.PluginOutput{Id: ids.PluginID{Category: "integration", Term: "integration name"}, Entity: entity.New("unique name", eID), Data: agent.PluginInventoryDataset{protocol.InventoryData{"id": "inventory_foo", "value": "bar"}}, NotApplicable: false})

	aCtx.On("SendEvent", mock.AnythingOfType("agent.mapEvent"), entity.Key("unique name"))

	aCtx.SendDataWg.Add(1)

	dmSender := &mockedMetricsSender{
		wg: sync.WaitGroup{},
	}
	dmSender.
		On("SendMetricsWithCommonAttributes", mock.AnythingOfType("protocol.Common"), mock.AnythingOfType("[]protocol.Metric")).
		Return(nil)
	dmSender.wg.Add(1)

	em := NewEmitter(aCtx, dmSender, &test.EmptyRegisterClient{})
	e := em.(*emitter)
	data := integrationFixture.ProtocolV4.Clone().ParsedV4
	// TODO update when key retrieval is fixed
	e.idCache.Put(entity.Key(data.DataSets[0].Entity.Name), eID)

	req := fwrequest.NewFwRequest(integration.Definition{}, nil, nil, data)

	em.Send(req)

	dmSender.wg.Wait()
	aCtx.SendDataWg.Wait()

	aCtx.AssertExpectations(t)

	// Should add Entity Id ('nr.entity.id') to Common attributes
	dmMetricsSent := dmSender.Calls[0].Arguments[1].([]protocol.Metric)
	assert.Len(t, dmMetricsSent, 1)
	assert.Equal(t, eID.String(), dmMetricsSent[0].Attributes[fwrequest.EntityIdAttribute])
}

func TestEmitter_Send(t *testing.T) {
	eID := entity.ID(1) // 1 as provided by test.NewIncrementalRegister

	aCtx := getAgentContext("bob")
	aCtx.On("SendData",
		agent.PluginOutput{Id: ids.PluginID{Category: "integration", Term: "integration name"}, Entity: entity.New("unique name", eID), Data: agent.PluginInventoryDataset{protocol.InventoryData{"id": "inventory_foo", "value": "bar"}, protocol.InventoryData{"entityKey": "unique name", "id": "integrationUser", "value": "root"}}, NotApplicable: false})

	aCtx.On("SendEvent", mock.Anything, entity.Key("unique name")).Run(assertEventData(t))

	aCtx.SendDataWg.Add(1)

	aCtx.On("Identity").Return(
		entity.Identity{
			ID: entity.ID(321), // agent one
		},
	)

	ms := &mockedMetricsSender{
		wg: sync.WaitGroup{},
	}
	ms.
		On("SendMetricsWithCommonAttributes", mock.AnythingOfType("protocol.Common"), mock.AnythingOfType("[]protocol.Metric")).
		Return(nil)
	ms.wg.Add(1)

	em := NewEmitter(aCtx, ms, test.NewIncrementalRegister())

	// avoid waiting for more data to create register submission batch
	e := em.(*emitter)
	e.registerMaxBatchSize = 1

	data := integrationFixture.ProtocolV4.Clone().ParsedV4
	em.Send(fwrequest.NewFwRequest(integration.Definition{ExecutorConfig: executor.Config{User: "root"}}, nil, nil, data))

	ms.wg.Wait()
	aCtx.SendDataWg.Wait()
	aCtx.AssertExpectations(t)

	// Should add Entity Id ('nr.entity.id') to Common attributes
	sent := ms.Calls[0].Arguments[1].([]protocol.Metric)
	assert.Len(t, sent, 1)
	assert.Equal(t, eID.String(), sent[0].Attributes[fwrequest.EntityIdAttribute])
}

func Test_NrEntityIdConst(t *testing.T) {
	assert.Equal(t, fwrequest.EntityIdAttribute, "nr.entity.id")
}

func TestEmitEvent_InvalidPayload(t *testing.T) {
	log.SetOutput(ioutil.Discard)  // discard logs so not to break race tests
	defer log.SetOutput(os.Stderr) // return back to default
	hook := new(logTest.Hook)
	log.AddHook(hook)
	log.SetLevel(logrus.WarnLevel)

	never := 0
	aCtx := getAgentContext("bob")
	aCtx.On("SendEvent").Times(never)

	d := integration.Definition{}
	plugin := agent.NewExternalPluginCommon(d.PluginID("integration.Name"), aCtx, "TestEmitEvent_InvalidPayload")

	emitEvent(&plugin, d, protocol.Dataset{Events: []protocol.EventData{{"value": "foo"}}}, nil, entity.ID(0))

	entry := hook.LastEntry()
	require.NotEmpty(t, hook.Entries)
	assert.Equal(t, "DimensionalMetricsEmitter", entry.Data["component"])
	assert.Equal(t, "discarding event, failed building event data.", entry.Message)
	assert.EqualError(t, entry.Data["error"].(error), "invalid event format: missing required 'summary' field")
	assert.Equal(t, logrus.WarnLevel, entry.Level)
}

func assertEventData(t *testing.T) func(args mock.Arguments) {
	return func(args mock.Arguments) {
		event := args.Get(0)
		plainEvent := fmt.Sprint(event)

		expectedSummary := "summary:foo"
		assert.Contains(t, plainEvent, expectedSummary)

		expectedEvent := "format:event"
		assert.Contains(t, plainEvent, expectedEvent)

		expectedCategory := "category:notifications"
		assert.Contains(t, plainEvent, expectedCategory)

		expectedType := "eventType:InfrastructureEvent"
		assert.Contains(t, plainEvent, expectedType)

		expectedEntityID := "entityID:1"
		assert.Contains(t, plainEvent, expectedEntityID)

		expectedAttribute := "attr.format:attribute"
		assert.Contains(t, plainEvent, expectedAttribute)

		expectedUser := "integrationUser:root"
		assert.Contains(t, plainEvent, expectedUser)
	}
}

func getAgentContext(hostname string) *mocks.AgentContext {
	agentCtx := &mocks.AgentContext{
		SendDataWg: sync.WaitGroup{},
	}
	idLookup := make(host.IDLookup)
	if hostname != "" {
		idLookup[sysinfo.HOST_SOURCE_INSTANCE_ID] = hostname
	}
	agentCtx.On("IDLookup").Return(idLookup)
	agentCtx.On("Config").Return(nil)

	return agentCtx
}
