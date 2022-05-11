// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package dm

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/instrumentation"

	"github.com/sirupsen/logrus"
	logTest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/fflag"
	"github.com/newrelic/infrastructure-agent/internal/agent/mocks"
	"github.com/newrelic/infrastructure-agent/internal/feature_flags"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/executor"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi/test"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/entity/host"
	"github.com/newrelic/infrastructure-agent/pkg/fwrequest"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo"
	integrationFixture "github.com/newrelic/infrastructure-agent/test/fixture/integration"
)

var (
	testIdentity = entity.Identity{
		ID:   1,
		GUID: "abcdef",
	}
)

func TestParsePayloadV4(t *testing.T) {
	ffm := feature_flags.NewManager(map[string]bool{fflag.FlagProtocolV4: true})

	d, err := ParsePayloadV4(integrationFixture.ProtocolV4.Payload, ffm)
	assert.NoError(t, err)
	assert.EqualValues(t, integrationFixture.ProtocolV4.ParsedV4, d)
}

func TestParsePayloadV4_embeddedInventoryItems(t *testing.T) {
	ffm := feature_flags.NewManager(map[string]bool{fflag.FlagProtocolV4: true})

	d, err := ParsePayloadV4([]byte(`{
  "protocol_version": "4",
  "integration": {
    "name": "com.newrelic.foo",
    "version": "0.1.0"
  },
  "data": [
    {
      "inventory": {
        "foo": {
          "bar": {
            "baz": {
              "k1": "v1",
              "k2": false
            }
          }
        }
      }
    }
  ]
}`), ffm)
	require.NoError(t, err)
	require.Len(t, d.DataSets, 1)

	// id: inventory data
	id := d.DataSets[0].Inventory

	fooID, ok := id["foo"]
	require.True(t, ok)

	barVal, ok := fooID["bar"]
	require.True(t, ok)
	barID, ok := barVal.(map[string]interface{})
	require.True(t, ok)

	bazVal, ok := barID["baz"]
	require.True(t, ok)
	bazID, ok := bazVal.(map[string]interface{})
	require.True(t, ok)

	k1Val, ok := bazID["k1"]
	require.True(t, ok)
	assert.EqualValues(t, "v1", k1Val)

	k2Val, ok := bazID["k2"]
	require.True(t, ok)
	assert.EqualValues(t, false, k2Val)
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
	data := integrationFixture.ProtocolV4TwoEntities.Clone().ParsedV4

	firstEntity := entity.Entity{Key: entity.Key(data.DataSets[0].Entity.Name), ID: entity.ID(1)}
	secondEntity := entity.Entity{Key: entity.Key(data.DataSets[1].Entity.Name), ID: entity.ID(2)}

	aCtx := getAgentContext("TestEmitter_Send_usingIDCache")
	aCtx.On("SendEvent", mock.Anything, mock.Anything)

	aCtx.On("SendData", agent.PluginOutput{Id: ids.PluginID{Category: "integration", Term: "Sample"}, Entity: firstEntity, Data: agent.PluginInventoryDataset{protocol.InventoryData{"id": "inventory_payload_one", "value": "foo-one"}}, NotApplicable: false})
	aCtx.On("SendData", agent.PluginOutput{Id: ids.PluginID{Category: "integration", Term: "Sample"}, Entity: secondEntity, Data: agent.PluginInventoryDataset{protocol.InventoryData{"id": "inventory_payload_two", "value": "bar-two"}}, NotApplicable: false})

	aCtx.SendDataWg.Add(2)

	dmSender := &mockedMetricsSender{
		wg: sync.WaitGroup{},
	}
	dmSender.
		On("SendMetricsWithCommonAttributes", mock.AnythingOfType("protocol.Common"), mock.AnythingOfType("[]protocol.Metric")).
		Return(nil)
	dmSender.wg.Add(2)

	em := NewEmitter(aCtx, dmSender, &test.EmptyRegisterClient{}, instrumentation.NoopMeasure)
	e := em.(*emitter)

	e.idCache.Put(entity.Key(fmt.Sprintf("%s:%s", data.DataSets[0].Entity.Type, data.DataSets[0].Entity.Name)), firstEntity.ID)
	e.idCache.Put(entity.Key(fmt.Sprintf("%s:%s", data.DataSets[1].Entity.Type, data.DataSets[1].Entity.Name)), secondEntity.ID)

	req := fwrequest.NewFwRequest(integration.Definition{}, nil, nil, data)

	em.Send(req)

	dmSender.wg.Wait()
	aCtx.SendDataWg.Wait()

	aCtx.AssertExpectations(t)

	// Should add Entity Id ('nr.entity.id') to Common attributes
	firstDMetricsSent := dmSender.Calls[0].Arguments[1].([]protocol.Metric)
	assert.Len(t, firstDMetricsSent, 1)
	assert.Equal(t, firstEntity.ID.String(), firstDMetricsSent[0].Attributes[fwrequest.EntityIdAttribute])

	secondDMetricsSent := dmSender.Calls[1].Arguments[1].([]protocol.Metric)
	assert.Len(t, secondDMetricsSent, 1)
	assert.Equal(t, secondEntity.ID.String(), secondDMetricsSent[0].Attributes[fwrequest.EntityIdAttribute])
}

func TestEmitter_Send_ignoreEntity(t *testing.T) {
	data := integrationFixture.ProtocolV4IgnoreEntity.Clone().ParsedV4

	aCtx := getAgentContext("TestEmitter_Send_ignoreEntity")

	dmSender := &mockedMetricsSender{
		wg: sync.WaitGroup{},
	}
	dmSender.
		On("SendMetricsWithCommonAttributes", mock.AnythingOfType("protocol.Common"), mock.AnythingOfType("[]protocol.Metric")).
		Return(nil)

	em := NewEmitter(aCtx, dmSender, &test.EmptyRegisterClient{}, instrumentation.NoopMeasure)

	dmSender.wg.Add(getMetricsSend(data))

	req := fwrequest.NewFwRequest(integration.Definition{}, nil, nil, data)

	em.Send(req)

	dmSender.wg.Wait()
	aCtx.SendDataWg.Wait()

	// Should not add Entity Id ('nr.entity.id') to Common attributes
	fmt.Println(dmSender.Calls[0].Arguments)
	firstDMetricsSent := dmSender.Calls[0].Arguments[1].([]protocol.Metric)
	assert.NotContains(t, firstDMetricsSent[0].Attributes, fwrequest.EntityIdAttribute)
}

func TestEmitter_Send(t *testing.T) {
	// configure mocks for emitter
	eID := entity.ID(1) // 1 as provided by test.NewIncrementalRegister

	aCtx := getAgentContext("TestEmitter_Send")
	aCtx.On("SendData",
		agent.PluginOutput{Id: ids.PluginID{Category: "integration", Term: "integration name"}, Entity: entity.New("unique name", eID), Data: agent.PluginInventoryDataset{protocol.InventoryData{"id": "inventory_foo", "value": "bar"}, protocol.InventoryData{"entityKey": "unique name", "id": "integrationUser", "value": "root"}}, NotApplicable: false})

	aCtx.On("SendEvent", mock.Anything, entity.Key("unique name")).Run(assertEventData(t))

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

	em := NewEmitter(aCtx, ms, test.NewIncrementalRegister(), instrumentation.NoopMeasure)

	// avoid waiting for more data to create register submission batch
	e := em.(*emitter)
	e.registerMaxBatchSize = 1

	// set tests cases
	var fwRequestTest = []struct {
		name               string
		integrationPayload protocol.DataV4
	}{
		{"Use host entity", integrationFixture.ProtocolV4.Clone().ParsedV4},
		{"Ignore entity", integrationFixture.ProtocolV4IgnoreEntity.Clone().ParsedV4},
	}

	for _, tt := range fwRequestTest {
		ms.wg.Add(getMetricsSend(tt.integrationPayload))
		aCtx.SendDataWg.Add(getInventoryToSend(tt.integrationPayload))
		em.Send(fwrequest.NewFwRequest(integration.Definition{ExecutorConfig: executor.Config{User: "root"}}, nil, nil, tt.integrationPayload))
		ms.wg.Wait()
		aCtx.SendDataWg.Wait()
		aCtx.AssertExpectations(t)

		for _, d := range tt.integrationPayload.DataSets {
			if !d.IgnoreEntity {
				entityName, err := d.Entity.Key()
				assert.NoError(t, err)
				actualEntityID, found := e.idCache.Get(entityName)
				assert.True(t, found)
				assert.Equal(t, eID, actualEntityID)
				assert.Equal(t, actualEntityID.String(), d.Common.Attributes[fwrequest.EntityIdAttribute])
			} else {
				entityName, err := d.Entity.Key()
				assert.NoError(t, err)
				assert.Equal(t, entity.EmptyKey, entityName)
				assert.NotContains(t, d.Common.Attributes, fwrequest.EntityIdAttribute)
			}

		}
	}
}

func TestEmitter_Send_failedToSubmitMetrics_dropAndLog(t *testing.T) {
	log.SetOutput(ioutil.Discard)  // discard logs so not to break race tests
	defer log.SetOutput(os.Stderr) // return back to default
	hook := new(logTest.Hook)
	log.AddHook(hook)
	log.SetLevel(logrus.WarnLevel)

	identity := entity.Identity{ID: entity.ID(321)}
	data := integrationFixture.ProtocolV4.Clone().ParsedV4

	ctx := getAgentContext("TestEmitter_Send_failedToSubmitMetrics")
	ctx.On("SendData", mock.Anything)
	ctx.On("SendEvent", mock.Anything, mock.Anything)
	ctx.On("Identity").Return(identity)
	ctx.SendDataWg.Add(1)

	ms := &mockedMetricsSender{wg: sync.WaitGroup{}}
	ms.On("SendMetricsWithCommonAttributes", mock.Anything, mock.Anything).Return(errors.New("failed to submit metrics"))
	ms.wg.Add(1)

	em := NewEmitter(ctx, ms, test.NewIncrementalRegister(), instrumentation.NoopMeasure).(*emitter)
	em.idCache.Put(entity.Key(fmt.Sprintf("%s:%s", data.DataSets[0].Entity.Type, data.DataSets[0].Entity.Name)), identity.ID)
	em.Send(fwrequest.NewFwRequest(integration.Definition{Name: "nri-test", ExecutorConfig: executor.Config{User: "root"}}, nil, nil, data))

	ms.wg.Wait()
	ctx.SendDataWg.Wait()

	var entry *logrus.Entry
	assert.Eventually(t, func() bool {
		entry = hook.LastEntry()
		return entry != nil
	}, time.Second, 10*time.Millisecond)

	require.NotNil(t, entry)
	assert.Equal(t, "DimensionalMetricsEmitter", entry.Data["component"])
	assert.Equal(t, "could not send metrics", entry.Message)
	assert.Equal(t, "nri-test", entry.Data["integration_name"])
	assert.EqualError(t, entry.Data["error"].(error), "failed to submit metrics")
	assert.Equal(t, logrus.WarnLevel, entry.Level, "expected for a Warn log level")
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
	aCtx := getAgentContext("TestEmitEvent_InvalidPayload")
	aCtx.On("SendEvent").Times(never)

	d := integration.Definition{}
	plugin := agent.NewExternalPluginCommon(d.PluginID("integration.Name"), aCtx, "TestEmitEvent_InvalidPayload")

	emitEvent(&plugin, d, protocol.Dataset{Events: []protocol.EventData{{"value": "foo"}}}, nil, nil, entity.ID(0))

	entry := hook.LastEntry()
	require.NotEmpty(t, hook.AllEntries())
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
	agentCtx.On("EntityKey").Return(hostname)
	agentCtx.On("IDLookup").Return(idLookup)
	agentCtx.On("Version").Return("dev")
	agentCtx.On("Config").Return(config.NewConfig())

	return agentCtx
}

// return the number of inventory data that will be sent by the agent
func getInventoryToSend(data protocol.DataV4) (toSend int) {
	for _, dataset := range data.DataSets {
		if len(dataset.Inventory) > 0 {
			toSend += 1
		}
	}
	return
}

// return the number of metrics that will be sent by the metrics sender
func getMetricsSend(data protocol.DataV4) (toSend int) {
	for _, dataset := range data.DataSets {
		if len(dataset.Metrics) > 0 {
			toSend += 1
		}
	}
	return
}
