// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package agent

import (
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi"

	"github.com/stretchr/testify/assert"

	backendhttp "github.com/newrelic/infrastructure-agent/pkg/backend/http"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/helpers/fingerprint"
)

var (
	testEntityId = entity.Identity{ID: 999666333}
)

type MockIdentityConnectClient struct {
}

func (icc *MockIdentityConnectClient) Connect(fp fingerprint.Fingerprint) (entity.Identity, backendhttp.RetryPolicy, error) {
	var retry backendhttp.RetryPolicy
	return testEntityId, retry, nil
}

func (icc *MockIdentityConnectClient) ConnectUpdate(entityID entity.Identity, fp fingerprint.Fingerprint) (backendhttp.RetryPolicy, entity.Identity, error) {
	var retry backendhttp.RetryPolicy
	return retry, testEntityId, nil
}

func (icc *MockIdentityConnectClient) Disconnect(entityID entity.ID, state identityapi.DisconnectReason) error {
	return nil
}

func TestConnect(t *testing.T) {
	service := NewIdentityConnectService(&MockIdentityConnectClient{}, &fingerprint.MockHarvestor{})

	assert.Equal(t, testEntityId, service.Connect())
}

func TestConnectUpdate(t *testing.T) {
	service := NewIdentityConnectService(&MockIdentityConnectClient{}, &fingerprint.MockHarvestor{})
	entityIdn, err := service.ConnectUpdate(entity.Identity{ID: 1})
	assert.NoError(t, err)
	assert.Equal(t, testEntityId, entityIdn)
}

func Test_ConnectUpdate_ReturnsSameIDForSameFingerprint(t *testing.T) {
	harvester := &fingerprint.MockHarvestor{}
	mockFingerprint, _ := harvester.Harvest()
	// explicitly setting null client to make sure we're not calling it IF we have the same fingerprint
	service := NewIdentityConnectService(nil, harvester)
	service.lastFingerprint = mockFingerprint

	agentIdn := entity.Identity{ID: 1}
	entityID, err := service.ConnectUpdate(agentIdn)

	assert.NoError(t, err)
	assert.Equal(t, entityID, agentIdn)
}

func Test_ConnectUpdate_ReturnsSameDifferentIDForDifferentFingerprint(t *testing.T) {
	harvester := &fingerprint.MockHarvestor{}
	mockFingerprint, _ := harvester.Harvest()
	mockFingerprint.Hostname = "someHostName"

	service := NewIdentityConnectService(&MockIdentityConnectClient{}, harvester)
	service.lastFingerprint = mockFingerprint

	agentIdn := entity.Identity{ID: 1}
	entityID, err := service.ConnectUpdate(agentIdn)

	assert.NoError(t, err)
	assert.Equal(t, testEntityId, entityID)
	assert.NotEqual(t, testEntityId, agentIdn)
}
