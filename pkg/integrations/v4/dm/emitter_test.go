// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package dm

import (
	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/handler"
	"github.com/newrelic/infrastructure-agent/internal/feature_flags"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo"
	integrationFixture "github.com/newrelic/infrastructure-agent/test/fixture/integration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
	"time"
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

type agentContext struct {
	agent.AgentContext
	id       entity.Identity
	idLookup agent.IDLookup
}

func (a *agentContext) AgentIdentity() entity.Identity {
	return a.id
}

func (a *agentContext) IDLookup() agent.IDLookup {
	return a.idLookup
}

func (a *agentContext) SendData(agent.PluginOutput) {

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

func (mk *mockedRegisterClient) RegisterProtocolEntities(agentEntityID entity.ID, entities []protocol.Entity,
) (identityapi.RegisterBatchEntityResponse, time.Duration, error) {

	args := mk.Called(agentEntityID, entities)
	return args.Get(0).(identityapi.RegisterBatchEntityResponse),
		args.Get(1).(time.Duration),
		args.Error(2)
}

func TestEmitter_Send(t *testing.T) {
	expectedEntityId := entity.ID(123)

	agentCtx := &agentContext{}
	agentCtx.id = entity.Identity{
		ID:   1,
		GUID: "abcdef",
	}

	agentCtx.idLookup = make(agent.IDLookup)
	agentCtx.idLookup[sysinfo.HOST_SOURCE_INSTANCE_ID] = "bob"

	dmSender := &mockedMetricsSender{}
	ffRetriever := &enabledFFRetriever{}
	registerClient := &mockedRegisterClient{}

	registerBatchEntityResponse := identityapi.RegisterBatchEntityResponse{{Name: "unique name", ID: expectedEntityId}}

	expectedEntities := []protocol.Entity{
		{
			Name:        "unique name",
			Type:        "RedisInstance",
			DisplayName: "human readable name",
			Metadata:    make(map[string]interface{}),
		}}
	registerClient.
		On("RegisterProtocolEntities", agentCtx.id.ID, expectedEntities).
		Return(registerBatchEntityResponse, time.Second, nil)

	dmSender.
		On("SendMetrics", mock.AnythingOfType("[]protocol.Metric"))

	emitter := NewEmitter(agentCtx, dmSender, ffRetriever, registerClient)

	metadata := integration.Definition{}
	var extraLabels data.Map
	var entityRewrite []data.EntityRewrite

	err := emitter.Send(metadata, extraLabels, entityRewrite, integrationFixture.ProtocolV4.Payload)

	assert.NoError(t, err)
	registerClient.AssertExpectations(t)
	dmSender.AssertExpectations(t)

	// Should add Entity Id ('nr.entity.id') to Common attributes
	dmMetricsSent := dmSender.Calls[0].Arguments[0].([]protocol.Metric)
	assert.Len(t, dmMetricsSent, 1)
	assert.Equal(t, expectedEntityId, dmMetricsSent[0].Attributes[nrEntityId])
}

func Test_NrEntityIdConst(t *testing.T) {
	assert.Equal(t, nrEntityId, "nr.entity.id")
}
