// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package agent

import (
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi/test"
	"github.com/newrelic/infrastructure-agent/pkg/backend/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
)

var registerEntities = []identityapi.RegisterEntity{identityapi.NewRegisterEntity("my-entity-1")}

func TestNewProvideIDs(t *testing.T) {
	provideIDs := NewProvideIDs(test.NewIncrementalRegister(), state.NewRegisterSM())

	ids, err := provideIDs(agentIdn, registerEntities)
	assert.NoError(t, err)

	require.Len(t, ids, 1)
	assert.Equal(t, registerEntities[0].Key, ids[0].Key)
	assert.Equal(t, entity.ID(1), ids[0].ID, "incremental register should return 1 as first id")
}

func TestRetryAfter(t *testing.T) {
	p := newIDProvider(test.NewRetryAfterRegister(), state.NewRegisterSM())

	_, err := p.provideIDs(agentIdn, registerEntities)
	assert.Error(t, err)
	assert.Equal(t, state.RegisterRetryAfter, p.state.State())
}

func TestRetryBackoff(t *testing.T) {
	p := newIDProvider(test.NewRetryBackoffRegister(), state.NewRegisterSM())

	_, err := p.provideIDs(agentIdn, registerEntities)
	assert.Error(t, err)
	assert.Equal(t, state.RegisterRetryBackoff, p.state.State())
}
