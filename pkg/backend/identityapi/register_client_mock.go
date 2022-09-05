// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package identityapi

import (
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/stretchr/testify/mock"
)

type RegisterClientMock struct {
	mock.Mock
}

func (r *RegisterClientMock) RegisterEntitiesRemoveMe(agentEntityID entity.ID, entities []RegisterEntity) ([]RegisterEntityResponse, time.Duration, error) {
	args := r.Called(agentEntityID, entities)

	return args.Get(0).([]RegisterEntityResponse), args.Get(1).(time.Duration), args.Error(2) // nolint
}

func (r *RegisterClientMock) ShouldRegisterEntitiesRemoveMe(agentEntityID entity.ID, entities []RegisterEntity, responses []RegisterEntityResponse, duration time.Duration) {
	r.
		On("RegisterEntitiesRemoveMe", agentEntityID, entities).
		Once().
		Return(responses, duration, nil)
}

func (r *RegisterClientMock) ShouldFailRegisterEntitiesRemoveMe(agentEntityID entity.ID, entities []RegisterEntity, duration time.Duration, err error) {
	r.
		On("RegisterEntitiesRemoveMe", agentEntityID, entities).
		Once().
		Return([]RegisterEntityResponse{}, duration, err)
}

func (r *RegisterClientMock) RegisterBatchEntities(agentEntityID entity.ID, entities []entity.Fields) ([]RegisterEntityResponse, error) {
	args := r.Called(agentEntityID, entities)

	return args.Get(0).([]RegisterEntityResponse), args.Error(1) // nolint
}

func (r *RegisterClientMock) ShouldRegisterBatchEntities(agentEntityID entity.ID, entities []entity.Fields, responses []RegisterEntityResponse) {
	r.
		On("RegisterBatchEntities", agentEntityID, entities).
		Once().
		Return(responses, nil)
}

func (r *RegisterClientMock) RegisterEntity(agentEntityID entity.ID, entity entity.Fields) (RegisterEntityResponse, error) {
	args := r.Called(agentEntityID, entity)

	return args.Get(0).(RegisterEntityResponse), args.Error(1) // nolint
}
