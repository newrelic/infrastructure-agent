// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package dm

import (
	"testing"

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
}

func (m *mockedMetricsSender) SendMetrics(metrics []protocol.Metric) {
	m.Called(metrics)
}

func (m *mockedMetricsSender) SendMetricsWithCommonAttributes(commonAttributes protocol.Common, metrics []protocol.Metric) error {
	return m.Called(commonAttributes, metrics).Error(0)
}

type mockedIdProvider struct {
	mock.Mock
}

func (mk *mockedIdProvider) ResolveEntities(entities []protocol.Entity) (registeredEntities registeredEntitiesNameToID, unregisteredEntitiesWithWait unregisteredEntityListWithWait) {
	args := mk.Called(entities)
	return args.Get(0).(registeredEntitiesNameToID),
		args.Get(1).(unregisteredEntityListWithWait)
}

func TestEmitter_Send_ErrorOnHostname(t *testing.T) {
	agentCtx := getAgentContext("")
	dmSender := &mockedMetricsSender{}
	idProvider := &mockedIdProvider{}

	idProvider.
		On("ResolveEntities", testIdentity, mock.Anything).
		Return(registeredEntitiesNameToID{}, unregisteredEntityList{})

	emitter := NewEmitter(agentCtx, dmSender, idProvider)

	metadata := integration.Definition{}
	var extraLabels data.Map
	var entityRewrite []data.EntityRewrite

	emitter.Send(NewDTO(metadata, extraLabels, entityRewrite, integrationFixture.ProtocolV4TwoEntities.ParsedV4))
	// TODO error handling
}

func TestEmitter_Send(t *testing.T) {
	expectedEntityId := "123"
	agentCtx := getAgentContext("bob")
	dmSender := &mockedMetricsSender{}
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
		On("ResolveEntities", expectedEntities).
		Return(
			registeredEntitiesNameToID{"unique name": entity.ID(123)},
			unregisteredEntityListWithWait{})
	dmSender.
		On("SendMetricsWithCommonAttributes", mock.AnythingOfType("protocol.Common"), mock.AnythingOfType("[]protocol.Metric")).
		Return(nil)

	agentCtx.On("SendData",
		agent.PluginOutput{Id: ids.PluginID{Category: "integration", Term: "integration name"}, Entity: entity.New("unique name", 123), Data: agent.PluginInventoryDataset{protocol.InventoryData{"id": "inventory_foo", "value": "bar"}}, NotApplicable: false})

	emitter := NewEmitter(agentCtx, dmSender, idProvider)

	metadata := integration.Definition{}
	var extraLabels data.Map
	var entityRewrite []data.EntityRewrite

	emitter.Send(NewDTO(metadata, extraLabels, entityRewrite, integrationFixture.ProtocolV4.ParsedV4))
	// TODO error handling
	idProvider.AssertExpectations(t)
	dmSender.AssertExpectations(t)
	agentCtx.AssertExpectations(t)

	// Should add Entity Id ('nr.entity.id') to Common attributes
	dmMetricsSent := dmSender.Calls[0].Arguments[1].([]protocol.Metric)
	assert.Len(t, dmMetricsSent, 1)
	assert.Equal(t, expectedEntityId, dmMetricsSent[0].Attributes[nrEntityId])
}

func getAgentContext(hostname string) *mocks.AgentContext {
	agentCtx := &mocks.AgentContext{}
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
