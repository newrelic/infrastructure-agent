// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package agent

import (
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
	"math/rand"
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

func (icc *EmptyRegisterClient) RegisterEntitiesRemoveMe(agentEntityID entity.ID, entities []identityapi.RegisterEntity) (r []identityapi.RegisterEntityResponse, retryAfter time.Duration, err error) {
	return
}

func (icc *EmptyRegisterClient) RegisterBatchEntities(agentEntityID entity.ID, entities []protocol.Entity) (r []identityapi.RegisterEntityResponse, retryAfter time.Duration, err error) {
	return
}

func (icc *EmptyRegisterClient) RegisterEntity(agentEntityID entity.ID, entity protocol.Entity) (resp identityapi.RegisterEntityResponse, err error) {
	return
}

type incrementalRegister struct {
	state state.Register
}

func newIncrementalRegister() identityapi.RegisterClient {
	return &incrementalRegister{state: state.RegisterHealthy}
}

func newRetryAfterRegister() identityapi.RegisterClient {
	return &incrementalRegister{state: state.RegisterRetryAfter}
}

func newRetryBackoffRegister() identityapi.RegisterClient {
	return &incrementalRegister{state: state.RegisterRetryBackoff}
}

func (r *incrementalRegister) RegisterBatchEntities(agentEntityID entity.ID, entities []protocol.Entity) (batchResponse []identityapi.RegisterEntityResponse, t time.Duration, err error) {
	return
}

func (r *incrementalRegister) RegisterEntitiesRemoveMe(agentEntityID entity.ID, entities []identityapi.RegisterEntity) (responseKeys []identityapi.RegisterEntityResponse, retryAfter time.Duration, err error) {
	if r.state == state.RegisterRetryAfter {
		retryAfter = 1 * time.Second
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

func (r *incrementalRegister) RegisterEntity(agentEntityID entity.ID, ent protocol.Entity) (identityapi.RegisterEntityResponse, error) {
	return identityapi.RegisterEntityResponse{
		ID:  entity.ID(rand.Int63n(100000)),
		Key: entity.Key(ent.Name),
	}, nil
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
