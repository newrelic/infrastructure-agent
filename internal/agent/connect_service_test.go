// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//nolint:exhaustruct
package agent

import (
	"errors"
	"regexp"
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/log" //nolint:depguard
	logHelper "github.com/newrelic/infrastructure-agent/test/log"
	"github.com/sirupsen/logrus"

	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi"

	"github.com/stretchr/testify/assert"

	backendhttp "github.com/newrelic/infrastructure-agent/pkg/backend/http"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/helpers/fingerprint"
)

// nolint:gochecknoglobals
var testEntityID = entity.Identity{ID: 999666333}

type MockIdentityConnectClient struct{}

func (icc *MockIdentityConnectClient) Connect(_ fingerprint.Fingerprint, _ identityapi.Metadata) (entity.Identity, backendhttp.RetryPolicy, error) {
	var retry backendhttp.RetryPolicy

	return testEntityID, retry, nil
}

func (icc *MockIdentityConnectClient) ConnectUpdate(_ entity.Identity, _ fingerprint.Fingerprint, _ identityapi.Metadata) (backendhttp.RetryPolicy, entity.Identity, error) {
	var retry backendhttp.RetryPolicy

	return retry, testEntityID, nil
}

func (icc *MockIdentityConnectClient) Disconnect(_ entity.ID, _ identityapi.DisconnectReason) error {
	return nil
}

func TestConnect(t *testing.T) {
	metadataHarvester := &identityapi.MetadataHarvesterMock{}
	defer metadataHarvester.AssertExpectations(t)
	metadataHarvester.ShouldHarvest(identityapi.Metadata{})

	service := NewIdentityConnectService(&MockIdentityConnectClient{}, &fingerprint.MockHarvestor{}, metadataHarvester)

	assert.Equal(t, testEntityID, service.Connect())
}

func TestConnectOnMetadataError(t *testing.T) {
	t.Parallel()

	metadataHarvester := &identityapi.MetadataHarvesterMock{}
	defer metadataHarvester.AssertExpectations(t)
	//nolint:goerr113
	metadataHarvester.ShouldNotHarvest(errors.New("some error"))
	metadataHarvester.ShouldHarvest(identityapi.Metadata{})

	// WHEN we add a hook to the log to capture the "error" and "fatal" levels
	hook := logHelper.NewInMemoryEntriesHook([]logrus.Level{logrus.ErrorLevel})
	log.AddHook(hook)

	service := NewIdentityConnectService(&MockIdentityConnectClient{}, &fingerprint.MockHarvestor{}, metadataHarvester)

	assert.Equal(t, testEntityID, service.Connect())
	assert.True(t, hook.EntryWithMessageExists(regexp.MustCompile(`metadata harvest failed`)))
}

func TestConnectUpdate(t *testing.T) {
	metadataHarvester := &identityapi.MetadataHarvesterMock{}
	defer metadataHarvester.AssertExpectations(t)
	metadataHarvester.ShouldHarvest(identityapi.Metadata{})

	service := NewIdentityConnectService(&MockIdentityConnectClient{}, &fingerprint.MockHarvestor{}, metadataHarvester)
	entityIdn, err := service.ConnectUpdate(entity.Identity{ID: 1})
	assert.NoError(t, err)
	assert.Equal(t, testEntityID, entityIdn)
}

func Test_ConnectUpdate_ReturnsSameIDForSameFingerprint(t *testing.T) {
	metadataHarvester := identityapi.MetadataHarvesterMock{}
	defer metadataHarvester.AssertExpectations(t)
	harvester := &fingerprint.MockHarvestor{}
	mockFingerprint, _ := harvester.Harvest()
	// explicitly setting null client to make sure we're not calling it IF we have the same fingerprint
	service := NewIdentityConnectService(nil, harvester, &metadataHarvester)
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

	metadataHarvester := identityapi.MetadataHarvesterMock{}
	defer metadataHarvester.AssertExpectations(t)

	metadataHarvester.ShouldHarvest(identityapi.Metadata{})

	service := NewIdentityConnectService(&MockIdentityConnectClient{}, harvester, &metadataHarvester)
	service.lastFingerprint = mockFingerprint

	agentIdn := entity.Identity{ID: 1}
	entityID, err := service.ConnectUpdate(agentIdn)

	assert.NoError(t, err)
	assert.Equal(t, testEntityID, entityID)
	assert.NotEqual(t, testEntityID, agentIdn)
}
