// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package dm

import (
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
	"github.com/newrelic/infrastructure-agent/pkg/entity/register"
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
	m.wg.Done()
	return m.Called(commonAttributes, metrics).Error(0)
}

type mockedIdProvider struct {
	mock.Mock
	wg sync.WaitGroup
}

func (mk *mockedIdProvider) ResolveEntities(entities []protocol.Entity) (registeredEntities register.RegisteredEntitiesNameToID, unregisteredEntitiesWithWait register.UnregisteredEntityListWithWait) {
	args := mk.Called(entities)
	mk.wg.Done()
	return args.Get(0).(register.RegisteredEntitiesNameToID),
		args.Get(1).(register.UnregisteredEntityListWithWait)
}

func TestEmitter_Send_usingIDCache(t *testing.T) {
	expectedEntityId := entity.ID(123)
	agentCtx := getAgentContext("bob")
	dmSender := &mockedMetricsSender{
		wg: sync.WaitGroup{},
	}
	dmSender.
		On("SendMetricsWithCommonAttributes", mock.AnythingOfType("protocol.Common"), mock.AnythingOfType("[]protocol.Metric")).
		Return(nil)
	dmSender.wg.Add(1)

	agentCtx.On("SendData",
		agent.PluginOutput{Id: ids.PluginID{Category: "integration", Term: "integration name"}, Entity: entity.New("unique name", expectedEntityId), Data: agent.PluginInventoryDataset{protocol.InventoryData{"id": "inventory_foo", "value": "bar"}}, NotApplicable: false})

	em := NewEmitter(agentCtx, dmSender, &test.EmptyRegisterClient{})
	e := em.(*emitter)
	payloadEntity := integrationFixture.ProtocolV4.ParsedV4.DataSets[0]
	// TODO update when key retrieval is fixed
	e.idCache.Put(entity.Key(payloadEntity.Entity.Name), expectedEntityId)

	req := fwrequest.NewFwRequest(integration.Definition{}, nil, nil, integrationFixture.ProtocolV4.ParsedV4)

	em.Send(req)

	dmSender.wg.Wait()

	agentCtx.AssertExpectations(t)

	// Should add Entity Id ('nr.entity.id') to Common attributes
	dmMetricsSent := dmSender.Calls[0].Arguments[1].([]protocol.Metric)
	assert.Len(t, dmMetricsSent, 1)
	assert.Equal(t, expectedEntityId.String(), dmMetricsSent[0].Attributes[fwrequest.EntityIdAttribute])
}

func TestEmitter_Send(t *testing.T) {
	agentCtx := getAgentContext("bob")

	dmSender := &mockedMetricsSender{
		wg: sync.WaitGroup{},
	}
	dmSender.
		On("SendMetricsWithCommonAttributes", mock.AnythingOfType("protocol.Common"), mock.AnythingOfType("[]protocol.Metric")).
		Return(nil)
	dmSender.wg.Add(1)

	eID := entity.ID(1) // 1 as provided by test.NewIncrementalRegister
	agentCtx.On("SendData",
		agent.PluginOutput{Id: ids.PluginID{Category: "integration", Term: "integration name"}, Entity: entity.New("unique name", eID), Data: agent.PluginInventoryDataset{protocol.InventoryData{"id": "inventory_foo", "value": "bar"}}, NotApplicable: false})
	agentCtx.On("Identity").Return(
		entity.Identity{
			ID: entity.ID(123), // agent one
		},
	)

	em := NewEmitter(agentCtx, dmSender, test.NewIncrementalRegister())

	// avoid waiting for more data to create register submission batch
	e := em.(*emitter)
	e.registerBatchSize = 1

	em.Send(fwrequest.NewFwRequest(integration.Definition{}, nil, nil, integrationFixture.ProtocolV4.ParsedV4))

	dmSender.wg.Wait()
	agentCtx.AssertExpectations(t)

	// Should add Entity Id ('nr.entity.id') to Common attributes
	dmMetricsSent := dmSender.Calls[0].Arguments[1].([]protocol.Metric)
	assert.Len(t, dmMetricsSent, 1)
	assert.Equal(t, "1", dmMetricsSent[0].Attributes[fwrequest.EntityIdAttribute])
}

func getAgentContext(hostname string) *mocks.AgentContext {
	agentCtx := &mocks.AgentContext{}
	idLookup := make(host.IDLookup)
	if hostname != "" {
		idLookup[sysinfo.HOST_SOURCE_INSTANCE_ID] = hostname
	}
	agentCtx.On("IDLookup").Return(idLookup)
	return agentCtx
}

func Test_NrEntityIdConst(t *testing.T) {
	assert.Equal(t, fwrequest.EntityIdAttribute, "nr.entity.id")
}
