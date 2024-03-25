// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package identityapi

import "github.com/stretchr/testify/mock"

type MetadataHarvesterMock struct {
	mock.Mock
}

func (m *MetadataHarvesterMock) Harvest() (Metadata, error) {
	args := m.Called()

	//nolint:forcetypeassert,wrapcheck
	return args.Get(0).(Metadata), args.Error(1)
}

func (m *MetadataHarvesterMock) ShouldHarvest(metadata Metadata) {
	m.On("Harvest").Once().Return(metadata, nil)
}

func (m *MetadataHarvesterMock) ShouldNotHarvest(err error) {
	m.On("Harvest").Once().Return(Metadata{}, err)
}
