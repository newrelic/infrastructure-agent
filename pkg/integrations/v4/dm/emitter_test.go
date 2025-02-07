// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package dm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent/types"

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
	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/entity/host"
	"github.com/newrelic/infrastructure-agent/pkg/fwrequest"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo"
	integrationFixture "github.com/newrelic/infrastructure-agent/test/fixture/integration"
	log2 "github.com/newrelic/infrastructure-agent/test/log"
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
	t.Parallel()
	ffm := feature_flags.NewManager(map[string]bool{})

	d, err := ParsePayloadV4(integrationFixture.ProtocolV4.Payload, ffm)
	assert.NoError(t, err)
	assert.EqualValues(t, integrationFixture.ProtocolV4.ParsedV4, d)
}

func TestParsePayloadV4_FF_disabled(t *testing.T) {
	t.Parallel()
	ffm := feature_flags.NewManager(map[string]bool{fflag.FlagProtocolV4: false})

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
	testCases := []struct {
		name      string
		FFEnabled bool
		FFExists  bool
	}{
		{
			name:      "dm_register_deprecated FF non existent not enabled",
			FFEnabled: false,
			FFExists:  false,
		},
		{
			name:      "dm_register_deprecated FF existent enabled (not possible but for safety)",
			FFEnabled: true,
			FFExists:  false,
		},
		{
			name:      "dm_register_deprecated FF existent and disabled",
			FFEnabled: false,
			FFExists:  true,
		},
	}

	for i := range testCases {
		testCase := testCases[i]
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			ffRetriever := &feature_flags.FeatureFlagRetrieverMock{}
			data := CopyProtocolParsingPair(t, integrationFixture.ProtocolV4TwoEntities).ParsedV4

			firstEntity := entity.Entity{Key: entity.Key(data.DataSets[0].Entity.Name), ID: entity.ID(1)}
			secondEntity := entity.Entity{Key: entity.Key(data.DataSets[1].Entity.Name), ID: entity.ID(2)}

			aCtx := getAgentContext("mock_reporting_agent")
			aCtx.On("SendEvent", mock.Anything, mock.Anything)

			aCtx.On("SendData", types.PluginOutput{
				Id:            ids.PluginID{Category: "integration", Term: "Sample"},
				Entity:        firstEntity,
				Data:          types.PluginInventoryDataset{protocol.InventoryData{"id": "inventory_payload_one", "value": "foo-one"}, protocol.InventoryData{"entityKey": "entity.name", "id": "integrationName", "value": "Sample"}, protocol.InventoryData{"entityKey": "entity.name", "id": "integrationVersion", "value": "1.2.3"}, protocol.InventoryData{"entityKey": "entity.name", "id": "reportingAgent", "value": "mock_reporting_agent"}},
				NotApplicable: false,
			})
			aCtx.On("SendData", types.PluginOutput{
				Id:            ids.PluginID{Category: "integration", Term: "Sample"},
				Entity:        secondEntity,
				Data:          types.PluginInventoryDataset{protocol.InventoryData{"id": "inventory_payload_two", "value": "bar-two"}, protocol.InventoryData{"entityKey": "entity.name", "id": "integrationName", "value": "Sample"}, protocol.InventoryData{"entityKey": "entity.name", "id": "integrationVersion", "value": "1.2.3"}, protocol.InventoryData{"entityKey": "entity.name", "id": "reportingAgent", "value": "mock_reporting_agent"}},
				NotApplicable: false,
			})

			dmSender := &mockedMetricsSender{
				wg: sync.WaitGroup{},
			}
			dmSender.
				On("SendMetricsWithCommonAttributes", mock.AnythingOfType("protocol.Common"), mock.AnythingOfType("[]protocol.Metric")).
				Return(nil)
			aCtx.SendDataWg.Add(2)
			dmSender.wg.Add(2)

			ffRetriever.ShouldGetFeatureFlag("dm_register_deprecated", testCase.FFEnabled, testCase.FFExists)
			ffRetriever.ShouldGetFeatureFlag("dm_register_deprecated", testCase.FFEnabled, testCase.FFExists)

			registerClient := &identityapi.RegisterClientMock{}
			emtr := NewEmitter(aCtx, dmSender, registerClient, ffRetriever)
			emtrStruct := emtr.(*emitter) // nolint:forcetypeassert

			emtrStruct.idCache.Put(entity.Key(fmt.Sprintf("%s:%s", data.DataSets[0].Entity.Type, data.DataSets[0].Entity.Name)), firstEntity.ID)
			emtrStruct.idCache.Put(entity.Key(fmt.Sprintf("%s:%s", data.DataSets[1].Entity.Type, data.DataSets[1].Entity.Name)), secondEntity.ID)

			req := fwrequest.NewFwRequest(integration.Definition{}, nil, nil, data)

			emtr.Send(req)

			dmSender.wg.Wait()
			aCtx.SendDataWg.Wait()

			// Should add Entity Id ('nr.entity.id') to Common attributes
			firstDMetricsSent := dmSender.Calls[0].Arguments[1].([]protocol.Metric) // nolint:forcetypeassert
			assert.Len(t, firstDMetricsSent, 1)
			assert.Equal(t, firstEntity.ID.String(), firstDMetricsSent[0].Attributes[fwrequest.EntityIdAttribute])

			secondDMetricsSent := dmSender.Calls[1].Arguments[1].([]protocol.Metric) // nolint:forcetypeassert
			assert.Len(t, secondDMetricsSent, 1)
			assert.Equal(t, secondEntity.ID.String(), secondDMetricsSent[0].Attributes[fwrequest.EntityIdAttribute])

			// Mocks expectations assertions
			mock.AssertExpectationsForObjects(t, ffRetriever, aCtx, registerClient)
		})
	}
}

func TestEmitter_Send_ignoreEntity(t *testing.T) {
	data := CopyProtocolParsingPair(t, integrationFixture.ProtocolV4IgnoreEntity).ParsedV4

	aCtx := &mocks.AgentContext{}
	aCtx.On("Config").Return(config.NewConfig())
	aCtx.On("Version").Return("dev")
	aCtx.On("EntityKey").Return("mock_reporting_agent")

	dmSender := &mockedMetricsSender{
		wg: sync.WaitGroup{},
	}
	dmSender.
		On("SendMetricsWithCommonAttributes", mock.AnythingOfType("protocol.Common"), mock.AnythingOfType("[]protocol.Metric")).
		Return(nil)

	ffRetriever := &feature_flags.FeatureFlagRetrieverMock{}

	registerClient := &identityapi.RegisterClientMock{}
	emtr := NewEmitter(aCtx, dmSender, registerClient, ffRetriever)

	dmSender.wg.Add(getMetricsSend(data))

	req := fwrequest.NewFwRequest(integration.Definition{}, nil, nil, data)

	emtr.Send(req)

	dmSender.wg.Wait()
	aCtx.SendDataWg.Wait()

	// Should not add Entity Id ('nr.entity.id') to Common attributes
	fmt.Println(dmSender.Calls[0].Arguments)
	firstDMetricsSent := dmSender.Calls[0].Arguments[1].([]protocol.Metric)
	assert.NotContains(t, firstDMetricsSent[0].Attributes, fwrequest.EntityIdAttribute)

	// Mocks expectations assertions
	mock.AssertExpectationsForObjects(t, ffRetriever, aCtx, registerClient)
}

func TestEmitter_Send(t *testing.T) {
	// set tests cases
	testCases := []struct {
		name               string
		integrationPayload protocol.DataV4
		ffExists           bool
		ffEnabled          bool
		register           bool
		tags               map[string]string
	}{
		{
			name:               "Use host entity",
			integrationPayload: CopyProtocolParsingPair(t, integrationFixture.ProtocolV4).ParsedV4,
			ffExists:           true,
			ffEnabled:          false,
			register:           true,
			tags: map[string]string{
				"test_tag_key":  "test_tag_value",
				"test_tag_key2": "test_tag_value2",
			},
		},
		{name: "Ignore entity no FF", integrationPayload: CopyProtocolParsingPair(t, integrationFixture.ProtocolV4IgnoreEntity).ParsedV4, ffExists: false, ffEnabled: false},
		{name: "Ignore entity no FF but enabled (not possible)", integrationPayload: CopyProtocolParsingPair(t, integrationFixture.ProtocolV4IgnoreEntity).ParsedV4, ffExists: false, ffEnabled: true},
		{name: "Ignore entity FF enabled", integrationPayload: CopyProtocolParsingPair(t, integrationFixture.ProtocolV4IgnoreEntity).ParsedV4, ffExists: true, ffEnabled: true},
	}

	for i := range testCases {
		testCase := testCases[i]
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			// configure mocks for emitter
			eID := entity.ID(1) // 1 as provided by test.NewIncrementalRegister
			agentEntityID := entity.ID(321)
			aCtx := &mocks.AgentContext{}
			if testCase.register {
				aCtx = getAgentContext("mock_reporting_agent")
				aCtx.On("SendData",
					types.PluginOutput{
						Id:     ids.PluginID{Category: "integration", Term: "integration name"},
						Entity: entity.New("unique name", eID),
						Data: types.PluginInventoryDataset{
							protocol.InventoryData{"id": "inventory_foo", "value": "bar"},
							protocol.InventoryData{"entityKey": "unique name", "id": "integrationUser", "value": "root"},
							protocol.InventoryData{"entityKey": "unique name", "id": "integrationName", "value": "integration name"},
							protocol.InventoryData{"entityKey": "unique name", "id": "integrationVersion", "value": "integration version"},
							protocol.InventoryData{"entityKey": "unique name", "id": "reportingAgent", "value": "mock_reporting_agent"},
						},
						NotApplicable: false,
					},
				)
				aCtx.On("SendEvent", mock.Anything, entity.Key("unique name")).Run(assertEventData(t))
				aCtx.On("Identity").Return(entity.Identity{ID: agentEntityID})
				aCtx.On("EntityKey").Return("mock_reporting_agent")
			} else {
				aCtx.On("Config").Return(config.NewConfig())
				aCtx.On("Version").Return("dev")
				aCtx.On("EntityKey").Return("mock_reporting_agent")
			}

			metricSender := &mockedMetricsSender{
				wg: sync.WaitGroup{},
			}
			metricSender.
				On("SendMetricsWithCommonAttributes", mock.AnythingOfType("protocol.Common"), mock.AnythingOfType("[]protocol.Metric")).
				Return(nil)

			ffRetriever := &feature_flags.FeatureFlagRetrieverMock{}
			registerClient := &identityapi.RegisterClientMock{}
			emtr := NewEmitter(aCtx, metricSender, registerClient, ffRetriever)

			// avoid waiting for more data to create register submission batch
			emtrStruct := emtr.(*emitter) // nolint:forcetypeassert
			emtrStruct.registerMaxBatchSize = 1

			if testCase.register {
				responses := []identityapi.RegisterEntityResponse{{ID: eID, Name: testCase.integrationPayload.DataSets[0].Entity.Name}}
				registerClient.ShouldRegisterBatchEntities(agentEntityID, []entity.Fields{testCase.integrationPayload.DataSets[0].Entity}, responses)
				ffRetriever.ShouldGetFeatureFlag("dm_register_deprecated", testCase.ffExists, testCase.ffEnabled)
			}
			metricSender.wg.Add(getMetricsSend(testCase.integrationPayload))
			aCtx.SendDataWg.Add(getInventoryToSend(testCase.integrationPayload))
			emtr.Send(fwrequest.NewFwRequest(integration.Definition{ExecutorConfig: executor.Config{User: "root"}, Tags: testCase.tags}, nil, nil, testCase.integrationPayload))
			metricSender.wg.Wait()
			aCtx.SendDataWg.Wait()
			for _, dataSet := range testCase.integrationPayload.DataSets {
				if !dataSet.IgnoreEntity {
					entityName, err := dataSet.Entity.Key()
					assert.NoError(t, err)
					actualEntityID, found := emtrStruct.idCache.Get(entityName)
					assert.True(t, found)
					assert.Equal(t, eID, actualEntityID)
					assert.Equal(t, actualEntityID.String(), dataSet.Common.Attributes[fwrequest.EntityIdAttribute])

					// Assert tags.
					for expectedTag, expectedTagVal := range testCase.tags {
						tagVal, found := testCase.integrationPayload.DataSets[0].Metrics[0].Attributes["tags."+expectedTag]
						assert.True(t, found)
						assert.Equal(t, expectedTagVal, tagVal)
					}
				} else {
					entityName, err := dataSet.Entity.Key()
					assert.NoError(t, err)
					assert.Equal(t, entity.EmptyKey, entityName)
					assert.NotContains(t, dataSet.Common.Attributes, fwrequest.EntityIdAttribute)
				}
			}
			// Mocks expectations assertions
			mock.AssertExpectationsForObjects(t, ffRetriever, aCtx, registerClient)
		})
	}
}

func TestEmitter_ShouldNotSendDataWhenDeprecatedRegisterFFIsEnabled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())

	aCtx := &mocks.AgentContext{}
	aCtx.On("Config").Return(config.NewConfig())
	aCtx.MockedContext = ctx

	hook := log2.NewInMemoryEntriesHook([]logrus.Level{logrus.WarnLevel})
	log.AddHook(hook)
	log.SetLevel(logrus.WarnLevel)

	ms := &mockedMetricsSender{}

	ffRetriever := &feature_flags.FeatureFlagRetrieverMock{}
	registerClient := &identityapi.RegisterClientMock{}
	emtr := NewEmitter(aCtx, ms, registerClient, ffRetriever)

	// avoid waiting for more data to create register submission batch
	emtrStruct := emtr.(*emitter) // nolint:forcetypeassert
	emtrStruct.registerMaxBatchSize = 1

	fwRequestTest := CopyProtocolParsingPair(t, integrationFixture.ProtocolV4DontIgnoreEntityIntegration).ParsedV4

	ffRetriever.ShouldGetExistingFeatureFlag("dm_register_deprecated", true)
	emtr.Send(fwrequest.NewFwRequest(integration.Definition{ExecutorConfig: executor.Config{User: "root"}}, nil, nil, fwRequestTest))
	time.Sleep(time.Millisecond * 50) // Give it time to parse the request before cancelling
	cancel()

	assert.Eventuallyf(t, func() bool {
		return emtrStruct.isProcessing.IsNotSet()
	}, time.Second, 10*time.Millisecond, "isProcessing should not be set")

	assert.Len(t, hook.GetEntries(), 1)
	assert.Equal(t, "Register for DM integrations is deprecated and therefore the data for this integration will not be sent. Check for the latest version of the integration.", hook.GetEntries()[0].Message)

	// Mocks expectations assertions
	mock.AssertExpectationsForObjects(t, ffRetriever, aCtx, registerClient)
}

func TestEmitter_ShouldRegisterFFDoesNotExistOrIsDisabled(t *testing.T) {
	t.Parallel()
	// set tests cases
	testCases := []struct {
		name               string
		integrationPayload protocol.DataV4
		FFExists           bool
		FFEnabled          bool
	}{
		{name: "Do not ignore entity, FF non existent", integrationPayload: CopyProtocolParsingPair(t, integrationFixture.ProtocolV4DontIgnoreEntityIntegration).ParsedV4, FFExists: false, FFEnabled: false},
		{name: "Do not ignore entity, FF disabled", integrationPayload: CopyProtocolParsingPair(t, integrationFixture.ProtocolV4DontIgnoreEntityIntegration).ParsedV4, FFExists: true, FFEnabled: false},
	}

	for i := range testCases {
		testCase := testCases[i]
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			identity := entity.Identity{ID: entity.ID(321)}
			// configure mocks for emitter
			aCtx := getAgentContext("localhost")
			aCtx.On("Identity").Return(identity)

			metricSender := &mockedMetricsSender{
				wg: sync.WaitGroup{},
			}
			metricSender.
				On("SendMetricsWithCommonAttributes", mock.AnythingOfType("protocol.Common"), mock.AnythingOfType("[]protocol.Metric")).
				Return(nil)

			ffRetriever := &feature_flags.FeatureFlagRetrieverMock{}

			registerClient := &identityapi.RegisterClientMock{}
			ent := testCase.integrationPayload.DataSets[0].Entity
			registerClient.ShouldRegisterBatchEntities(identity.ID, []entity.Fields{ent}, []identityapi.RegisterEntityResponse{{ID: identity.ID, Name: ent.Name}})

			emtr := NewEmitter(aCtx, metricSender, registerClient, ffRetriever)

			// avoid waiting for more data to create register submission batch
			emtrStruct := emtr.(*emitter) // nolint:forcetypeassert
			emtrStruct.registerMaxBatchSize = 1

			ffRetriever.ShouldGetFeatureFlag("dm_register_deprecated", testCase.FFExists, testCase.FFEnabled)
			metricSender.wg.Add(getMetricsSend(testCase.integrationPayload))
			aCtx.SendDataWg.Add(getInventoryToSend(testCase.integrationPayload))
			emtr.Send(fwrequest.NewFwRequest(integration.Definition{ExecutorConfig: executor.Config{User: "root"}}, nil, nil, testCase.integrationPayload))
			metricSender.wg.Wait()
			aCtx.SendDataWg.Wait()

			// Mocks expectations assertions
			mock.AssertExpectationsForObjects(t, ffRetriever, aCtx, registerClient)
		})
	}
}

func TestEmitter_Send_failedToSubmitMetrics_dropAndLog(t *testing.T) {
	log.SetOutput(ioutil.Discard)  // discard logs so not to break race tests
	defer log.SetOutput(os.Stderr) // return back to default
	hook := new(logTest.Hook)
	log.AddHook(hook)
	log.SetLevel(logrus.WarnLevel)

	identity := entity.Identity{ID: entity.ID(321)}
	data := CopyProtocolParsingPair(t, integrationFixture.ProtocolV4).ParsedV4

	aCtx := getAgentContext("TestEmitter_Send_failedToSubmitMetrics")
	aCtx.On("SendData", mock.Anything)
	aCtx.On("SendEvent", mock.Anything, mock.Anything)
	aCtx.SendDataWg.Add(1)

	ms := &mockedMetricsSender{wg: sync.WaitGroup{}}
	ms.On("SendMetricsWithCommonAttributes", mock.Anything, mock.Anything).Return(errors.New("failed to submit metrics"))
	ms.wg.Add(1)

	ffRetriever := &feature_flags.FeatureFlagRetrieverMock{}
	ffRetriever.ShouldNotGetFeatureFlag("dm_register_deprecated")

	registerClient := &identityapi.RegisterClientMock{}
	emtr := NewEmitter(aCtx, ms, registerClient, ffRetriever).(*emitter) //nolint:forcetypeassert
	emtr.idCache.Put(entity.Key(fmt.Sprintf("%s:%s", data.DataSets[0].Entity.Type, data.DataSets[0].Entity.Name)), identity.ID)
	emtr.Send(fwrequest.NewFwRequest(integration.Definition{Name: "nri-test", ExecutorConfig: executor.Config{User: "root"}}, nil, nil, data))

	ms.wg.Wait()
	aCtx.SendDataWg.Wait()

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

	// Mocks expectations assertions
	mock.AssertExpectationsForObjects(t, ffRetriever, aCtx, registerClient)
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

// aims to avoid data mutation when processing structs from global fixture variables.
func CopyProtocolParsingPair(t *testing.T, p integrationFixture.ProtocolParsingPair) (clone integrationFixture.ProtocolParsingPair) {
	t.Helper()
	m, err := json.Marshal(p)
	require.NoError(t, err)
	err = json.Unmarshal(m, &clone)
	require.NoError(t, err)

	return
}
