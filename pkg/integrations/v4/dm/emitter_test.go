// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package dm

import (
	"errors"
	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/handler"
	"github.com/newrelic/infrastructure-agent/internal/agent/mocks"
	"github.com/newrelic/infrastructure-agent/internal/feature_flags"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo"
	integrationFixture "github.com/newrelic/infrastructure-agent/test/fixture/integration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
	"time"
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

type mockedRegisterClient struct {
	identityapi.RegisterClient
	mock.Mock
}

func (mk *mockedRegisterClient) RegisterBatchEntities(agentEntityID entity.ID, entities []protocol.Entity,
) ([]identityapi.RegisterEntityResponse, time.Duration, error) {

	args := mk.Called(agentEntityID, entities)
	return args.Get(0).([]identityapi.RegisterEntityResponse),
		args.Get(1).(time.Duration),
		args.Error(2)
}

func TestEmitter_Send_RegisterErr(t *testing.T) {
	agentCtx := getAgentContext("bob")
	dmSender := &mockedMetricsSender{}
	ffRetriever := &enabledFFRetriever{}
	registerClient := &mockedRegisterClient{}

	expectedError := errors.New("expected error")
	registerClient.
		On("RegisterBatchEntities", testIdentity.ID, mock.Anything).
		Return([]identityapi.RegisterEntityResponse{}, time.Second, expectedError)

	emitter := NewEmitter(agentCtx, dmSender, ffRetriever, registerClient)
	metadata := integration.Definition{}
	var extraLabels data.Map
	var entityRewrite []data.EntityRewrite

	err := emitter.Send(metadata, extraLabels, entityRewrite, integrationFixture.ProtocolV4.Payload)

	assert.EqualError(t, err, expectedError.Error())
}

func TestEmitter_Send_ErrorOnHostname(t *testing.T) {
	expectedEntityId := entity.ID(123)
	agentCtx := getAgentContext("")
	dmSender := &mockedMetricsSender{}
	ffRetriever := &enabledFFRetriever{}
	registerClient := &mockedRegisterClient{}

	registerBatchEntityResponse := []identityapi.RegisterEntityResponse{{Name: "unique name", ID: expectedEntityId}}

	expectedEntities := []protocol.Entity{
		{Name: "a.entity.one", Type: "ATYPE", DisplayName: "A display name one", Metadata: map[string]interface{}{"env": "testing"}},
		{Name: "b.entity.two", Type: "ATYPE", DisplayName: "A display name two", Metadata: map[string]interface{}{"env": "testing"}},
	}
	registerClient.
		On("RegisterBatchEntities", testIdentity.ID, expectedEntities).
		Return(registerBatchEntityResponse, time.Second, nil)

	emitter := NewEmitter(agentCtx, dmSender, ffRetriever, registerClient)

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
	registerClient := &mockedRegisterClient{}

	registerBatchEntityResponse := []identityapi.RegisterEntityResponse{
		{
			Name: "A display name one",
			ID:   expectedEntityId,
			Key:  "a.entity.one",
		},
	}

	expectedEntities := []protocol.Entity{
		{Name: "a.entity.one", Type: "ATYPE", DisplayName: "A display name one", Metadata: map[string]interface{}{"env": "testing"}},
		{Name: "b.entity.two", Type: "ATYPE", DisplayName: "A display name two", Metadata: map[string]interface{}{"env": "testing"}},
	}
	registerClient.
		On("RegisterBatchEntities", testIdentity.ID, expectedEntities).
		Return(registerBatchEntityResponse, time.Second, nil)

	dmSender.
		On("SendMetrics", mock.AnythingOfType("[]protocol.Metric"))

	agentCtx.On("SendData",
		agent.PluginOutput{
			Id:        ids.PluginID{Category: "integration", Term: "Sample"},
			EntityKey: "a.entity.one",
			Data: agent.PluginInventoryDataset{
				protocol.InventoryData{"id": "inventory_payload_one", "value": "foo-one"},
			}, NotApplicable: false})

	emitter := NewEmitter(agentCtx, dmSender, ffRetriever, registerClient)

	metadata := integration.Definition{}
	var extraLabels data.Map
	var entityRewrite []data.EntityRewrite

	err := emitter.Send(metadata, extraLabels, entityRewrite, integrationFixture.ProtocolV4TwoEntities.Payload)
	assert.EqualError(t, err, "1 out of 2 datasets could not be emitted. Reasons: entity with name 'b.entity.two' was not registered in the backend")

	registerClient.AssertExpectations(t)
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
	registerClient := &mockedRegisterClient{}

	registerBatchEntityResponse := []identityapi.RegisterEntityResponse{
		{
			Name: "unique name",
			ID:   expectedEntityId,
			Key:  "unique name",
		},
	}

	expectedEntities := []protocol.Entity{
		{
			Name:        "unique name",
			Type:        "RedisInstance",
			DisplayName: "human readable name",
			Metadata:    make(map[string]interface{}),
		}}
	registerClient.
		On("RegisterBatchEntities", testIdentity.ID, expectedEntities).
		Return(registerBatchEntityResponse, time.Second, nil)

	dmSender.
		On("SendMetrics", mock.AnythingOfType("[]protocol.Metric"))

	agentCtx.On("SendData",
		agent.PluginOutput{Id: ids.PluginID{Category: "integration", Term: "integration name"}, EntityKey: "unique name", Data: agent.PluginInventoryDataset{protocol.InventoryData{"id": "inventory_foo", "value": "bar"}}, NotApplicable: false})

	emitter := NewEmitter(agentCtx, dmSender, ffRetriever, registerClient)

	metadata := integration.Definition{}
	var extraLabels data.Map
	var entityRewrite []data.EntityRewrite

	err := emitter.Send(metadata, extraLabels, entityRewrite, integrationFixture.ProtocolV4.Payload)

	assert.NoError(t, err)
	registerClient.AssertExpectations(t)
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
