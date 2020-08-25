// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package dm

import (
	"fmt"
	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/handler"
	"github.com/newrelic/infrastructure-agent/internal/agent/mocks"
	"github.com/newrelic/infrastructure-agent/internal/feature_flags"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo"
	integrationFixture "github.com/newrelic/infrastructure-agent/test/fixture/integration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
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
	MetricsSender
	mock.Mock
}

func (m *mockedMetricsSender) SendMetrics(metrics []protocol.Metric) {
	m.Called(metrics)
}

type enabledFFRetriever struct{}

func (e *enabledFFRetriever) GetFeatureFlag(name string) (enabled bool, exists bool) {
	return true, true
}

type mockedIdProvider struct {
	mock.Mock
}

func (mk *mockedIdProvider) Entities(agentIdn entity.Identity, entities []protocol.Entity) (registeredEntities RegisteredEntitiesNameToID, unregisteredEntities UnregisteredEntities) {
	args := mk.Called(agentIdn, entities)
	return args.Get(0).(RegisteredEntitiesNameToID),
		args.Get(1).(UnregisteredEntities)
}

func TestEmitter_Send_ErrorOnHostname(t *testing.T) {
	agentCtx := getAgentContext("")
	dmSender := &mockedMetricsSender{}
	ffRetriever := &enabledFFRetriever{}
	idProvider := &mockedIdProvider{}

	idProvider.
		On("Entities", testIdentity, mock.Anything).
		Return(RegisteredEntitiesNameToID{}, UnregisteredEntities{})

	emitter := NewEmitter(agentCtx, dmSender, ffRetriever, idProvider)

	metadata := integration.Definition{}
	var extraLabels data.Map
	var entityRewrite []data.EntityRewrite

	err := emitter.Send(metadata, extraLabels, entityRewrite, integrationFixture.ProtocolV4TwoEntities.Payload)
	assert.EqualError(t, err, "2 out of 2 datasets could not be emitted. Reasons: error renaming entity: no known identifier types found in ID lookup table")
}

func TestEmitter_SendOneEntityOutOfTwo(t *testing.T) {
	expectedEntityId := entity.ID(123)
	agentCtx := getAgentContext("test")
	dmSender := &mockedMetricsSender{}
	ffRetriever := &enabledFFRetriever{}
	idProvider := &mockedIdProvider{}

	expectedEntities := []protocol.Entity{
		{Name: "a.entity.one", Type: "ATYPE", DisplayName: "A display name one", Metadata: map[string]interface{}{"env": "testing"}},
		{Name: "b.entity.two", Type: "ATYPE", DisplayName: "A display name two", Metadata: map[string]interface{}{"env": "testing"}},
	}

	idProvider.
		On("Entities", testIdentity, expectedEntities).
		Return(
			RegisteredEntitiesNameToID{"a.entity.one": expectedEntityId},
			UnregisteredEntities{
				{
					Reason: reasonEntityError,
					Err:    fmt.Errorf("invalid entityName"),
					Entity: protocol.Entity{
						Name:        "b.entity.two",
						Type:        "ATYPE",
						DisplayName: "A display name two",
						Metadata:    map[string]interface{}{"env": "testing"},
					},
				},
			})

	dmSender.
		On("SendMetrics", mock.AnythingOfType("[]protocol.Metric"))

	agentCtx.On("SendData",
		agent.PluginOutput{
			Id:        ids.PluginID{Category: "integration", Term: "Sample"},
			EntityKey: "a.entity.one",
			Data: agent.PluginInventoryDataset{
				protocol.InventoryData{"id": "inventory_payload_one", "value": "foo-one"},
			}, NotApplicable: false})

	emitter := NewEmitter(agentCtx, dmSender, ffRetriever, idProvider)

	metadata := integration.Definition{}
	var extraLabels data.Map
	var entityRewrite []data.EntityRewrite

	err := emitter.Send(metadata, extraLabels, entityRewrite, integrationFixture.ProtocolV4TwoEntities.Payload)
	assert.EqualError(t, err, "1 out of 2 datasets could not be emitted. Reasons: entity with name 'b.entity.two' was not registered in the backend")

	idProvider.AssertExpectations(t)
	dmSender.AssertExpectations(t)
	agentCtx.AssertExpectations(t)

	// Should add Entity Id ('nr.entity.id') to Common attributes
	dmMetricsSent := dmSender.Calls[0].Arguments[0].([]protocol.Metric)
	assert.Len(t, dmMetricsSent, 3)
	assert.Equal(t, expectedEntityId, dmMetricsSent[0].Attributes[nrEntityId])
	assert.Equal(t, expectedEntityId, dmMetricsSent[1].Attributes[nrEntityId])
	assert.Equal(t, expectedEntityId, dmMetricsSent[2].Attributes[nrEntityId])
}

func TestEmitter_Send(t *testing.T) {
	expectedEntityId := entity.ID(123)
	agentCtx := getAgentContext("bob")
	dmSender := &mockedMetricsSender{}
	ffRetriever := &enabledFFRetriever{}
	idProvider := &mockedIdProvider{}

	expectedEntities := []protocol.Entity{
		{
			Name:        "unique name",
			Type:        "RedisInstance",
			DisplayName: "human readable name",
			Metadata:    map[string]interface{}{},
		},
	}

	idProvider.
		On("Entities", testIdentity, expectedEntities).
		Return(
			RegisteredEntitiesNameToID{"unique name": expectedEntityId},
			UnregisteredEntities{})
	dmSender.
		On("SendMetrics", mock.AnythingOfType("[]protocol.Metric"))

	agentCtx.On("SendData",
		agent.PluginOutput{Id: ids.PluginID{Category: "integration", Term: "integration name"}, EntityKey: "unique name", Data: agent.PluginInventoryDataset{protocol.InventoryData{"id": "inventory_foo", "value": "bar"}}, NotApplicable: false})

	emitter := NewEmitter(agentCtx, dmSender, ffRetriever, idProvider)

	metadata := integration.Definition{}
	var extraLabels data.Map
	var entityRewrite []data.EntityRewrite

	err := emitter.Send(metadata, extraLabels, entityRewrite, integrationFixture.ProtocolV4.Payload)

	assert.NoError(t, err)
	idProvider.AssertExpectations(t)
	dmSender.AssertExpectations(t)
	agentCtx.AssertExpectations(t)

	// Should add Entity Id ('nr.entity.id') to Common attributes
	dmMetricsSent := dmSender.Calls[0].Arguments[0].([]protocol.Metric)
	assert.Len(t, dmMetricsSent, 1)
	assert.Equal(t, expectedEntityId, dmMetricsSent[0].Attributes[nrEntityId])
}

func getAgentContext(hostname string) *mocks.AgentContext {
	agentCtx := &mocks.AgentContext{}
	agentCtx.On("AgentIdentity").Return(testIdentity)
	idLookup := make(agent.IDLookup)
	if hostname != "" {
		idLookup[sysinfo.HOST_SOURCE_INSTANCE_ID] = hostname
	}
	agentCtx.On("IDLookup").Return(idLookup)
	return agentCtx
}

func Test_NrEntityIdConst(t *testing.T) {
	assert.Equal(t, nrEntityId, "nr.entity.id")
}
