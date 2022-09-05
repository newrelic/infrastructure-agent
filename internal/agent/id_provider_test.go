// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package agent

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi"
	"github.com/newrelic/infrastructure-agent/pkg/backend/state"
)

var registerEntities = []identityapi.RegisterEntity{identityapi.NewRegisterEntity("my-entity-1")}

func TestNewProvideIDs(t *testing.T) {
	registerClient := &identityapi.RegisterClientMock{}
	provideIDs := NewProvideIDs(registerClient, state.NewRegisterSM())

	registeredEntityResponse := identityapi.RegisterEntityResponse{ID: agentIdn.ID, Name: "some name"}

	duration := time.Second
	registerClient.ShouldRegisterEntitiesRemoveMe(agentIdn.ID, registerEntities, []identityapi.RegisterEntityResponse{registeredEntityResponse}, duration)
	ids, err := provideIDs(agentIdn, registerEntities)
	assert.NoError(t, err)

	require.Len(t, ids, 1)
	assert.Equal(t, registeredEntityResponse.Name, ids[0].Name)
	assert.Equal(t, registeredEntityResponse.ID, ids[0].ID)
}

func TestRetryAfter(t *testing.T) {
	registerClient := &identityapi.RegisterClientMock{}
	retryAfterError := errors.New("retry after") // nolint:goerr113
	retry := time.Second                         // positive time means a retry
	registerClient.ShouldFailRegisterEntitiesRemoveMe(agentIdn.ID, registerEntities, retry, retryAfterError)

	p := newIDProvider(registerClient, state.NewRegisterSM())

	_, err := p.provideIDs(agentIdn, registerEntities)
	assert.Error(t, err, retryAfterError.Error())
	assert.Equal(t, state.RegisterRetryAfter, p.state.State())
	registerClient.AssertExpectations(t)
}

func TestRetryBackoff(t *testing.T) {
	registerClient := &identityapi.RegisterClientMock{}
	backoffError := errors.New("backoff") // nolint:goerr113
	retry := time.Second * 0              // non-positive time means a backoff
	registerClient.ShouldFailRegisterEntitiesRemoveMe(agentIdn.ID, registerEntities, retry, backoffError)
	p := newIDProvider(registerClient, state.NewRegisterSM())

	_, err := p.provideIDs(agentIdn, registerEntities)
	assert.Error(t, err, backoffError)
	assert.Equal(t, state.RegisterRetryBackoff, p.state.State())
}
