// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package hostid

import "github.com/stretchr/testify/mock"

type ProviderMock struct {
	mock.Mock
}

func (m *ProviderMock) Provide() (string, error) {
	args := m.Called()

	return args.String(0), args.Error(1)
}

func (m *ProviderMock) ShouldProvide(hostID string) {
	m.On("Provide").Once().Return(hostID, nil)
}

func (m *ProviderMock) ShouldReturnErr(err error) {
	m.On("Provide").Once().Return("", err)
}
