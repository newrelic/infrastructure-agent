// Copyright 2023 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"errors"
	"fmt"
	"strconv"
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"
)

type MockHarvester struct {
	mockType        cloud.Type
	retryCount      int
	getIDOnFirstTry bool
}

var ErrUnexpected = errors.New("an unexpected error")

func NewMockHarvester(t *testing.T, mockType cloud.Type, getIDOnFirstTry bool) *MockHarvester {
	t.Helper()

	return &MockHarvester{
		mockType:        mockType,
		retryCount:      0,
		getIDOnFirstTry: getIDOnFirstTry,
	}
}

// GetInstanceID will return the id of the cloud instance.
func (m *MockHarvester) GetInstanceID() (string, error) {
	m.retryCount++

	if m.getIDOnFirstTry {
		return "Got cloud ID on first try!", nil
	}

	if m.mockType.IsValidCloud() && m.retryCount == 3 {
		return "Got cloud ID on try " + strconv.Itoa(m.retryCount), nil
	}

	return "", fmt.Errorf("%w", ErrUnexpected)
}

// GetHostType will return the cloud instance type.
func (m *MockHarvester) GetHostType() (string, error) {
	return "test host type", nil
}

// GetCloudType will return the cloud type on which the instance is running.
func (m *MockHarvester) GetCloudType() cloud.Type {
	return m.mockType
}

// Returns a string key which will be used as a HostSource (see host_aliases plugin).
func (m *MockHarvester) GetCloudSource() string {
	return "test cloud source"
}

// GetRegion returns the region where the instance is running.
func (m *MockHarvester) GetRegion() (string, error) {
	return "myRegion", nil
}

// GetAccountID returns the cloud account where the instance is running.
func (m *MockHarvester) GetAccountID() (string, error) {
	return "", nil
}

// GetInstanceImageID returns the instance image ID.
func (m *MockHarvester) GetInstanceImageID() (string, error) {
	return "", nil
}

// GetZone returns the instance cloud zone.
func (m *MockHarvester) GetZone() (string, error) {
	return "", nil
}

// GetInstanceDisplayName returns the cloud instance display name.
func (m *MockHarvester) GetInstanceDisplayName() (string, error) {
	return "", nil
}

// GetInstanceTenantID returns the cloud instance tenant ID.
func (m *MockHarvester) GetInstanceTenantID() (string, error) {
	return "", nil
}

// GetHarvester returns the MockHarvester.
func (m *MockHarvester) GetHarvester() (cloud.Harvester, error) { //nolint: ireturn
	return m, nil
}
