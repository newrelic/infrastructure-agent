// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package agent

import (
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/backend/inventoryapi"
	"github.com/newrelic/infrastructure-agent/pkg/backend/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"net/http"

	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
)

type EmptyRegisterClient struct{}

var registerEntities = []identityapi.RegisterEntity{identityapi.NewRegisterEntity("my-entity-1")}

func (icc *EmptyRegisterClient) Register(agentEntityID entity.ID, entities []identityapi.RegisterEntity) (r []identityapi.RegisterEntityResponse, retryAfter time.Duration, err error) {
	return
}

type incrementalRegister struct {
	state state.Register
}

func newIncrementalRegister() identityapi.IdentityRegisterClient {
	return &incrementalRegister{state: state.RegisterHealthy}
}

func newRetryAfterRegister() identityapi.IdentityRegisterClient {
	return &incrementalRegister{state: state.RegisterRetryAfter}
}

func newRetryBackoffRegister() identityapi.IdentityRegisterClient {
	return &incrementalRegister{state: state.RegisterRetryBackoff}
}

func (r *incrementalRegister) Register(agentEntityID entity.ID, entities []identityapi.RegisterEntity) (responseKeys []identityapi.RegisterEntityResponse, retryAfter time.Duration, err error) {
	if r.state == state.RegisterRetryAfter {
		retryAfter = time.Duration(1 * time.Second)
		err = inventoryapi.NewIngestError("ingest service rejected the register step", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), "")
		return
	} else if r.state == state.RegisterRetryBackoff {
		err = inventoryapi.NewIngestError("ingest service rejected the register step", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), "")
		return
	}

	var i entity.ID
	for _, e := range entities {
		i++
		responseKeys = append(responseKeys, identityapi.RegisterEntityResponse{ID: i, Key: e.Key})
	}

	return
}

func TestNewProvideIDs(t *testing.T) {
	provideIDs := NewProvideIDs(newIncrementalRegister(), state.NewRegisterSM())

	ids, err := provideIDs(agentIdn, registerEntities)
	assert.NoError(t, err)

	require.Len(t, ids, 1)
	assert.Equal(t, registerEntities[0].Key, ids[0].Key)
	assert.Equal(t, entity.ID(1), ids[0].ID, "incremental register should return 1 as first id")
}

func TestRetryAfter(t *testing.T) {
	p := newIDProvider(newRetryAfterRegister(), state.NewRegisterSM())

	_, err := p.provideIDs(agentIdn, registerEntities)
	assert.Error(t, err)
	assert.Equal(t, state.RegisterRetryAfter, p.state.State())
}

func TestRetryBackoff(t *testing.T) {
	p := newIDProvider(newRetryBackoffRegister(), state.NewRegisterSM())

	_, err := p.provideIDs(agentIdn, registerEntities)
	assert.Error(t, err)
	assert.Equal(t, state.RegisterRetryBackoff, p.state.State())
}
