// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//
//nolint:exhaustruct
package metrics

import (
	"encoding/json"
	"errors"
	"regexp"
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/log"

	logHelper "github.com/newrelic/infrastructure-agent/test/log"

	"github.com/newrelic/infrastructure-agent/internal/agent/mocks"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/storage"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/hostid"
	"github.com/stretchr/testify/assert"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestNewSystemSampler(t *testing.T) {
	ctx := new(mocks.AgentContext)
	ctx.On("Config").Return(&config.Config{})

	hostIDProvider := &hostid.ProviderMock{} //nolint:exhaustruct
	defer hostIDProvider.AssertExpectations(t)

	m := NewSystemSampler(ctx, nil, nil, hostIDProvider)

	assert.NotNil(t, m)
}

func TestSystemSample(t *testing.T) {
	ctx := new(mocks.AgentContext)
	ctx.On("Config").Return(&config.Config{})

	hostIDProvider := &hostid.ProviderMock{} //nolint:exhaustruct
	defer hostIDProvider.AssertExpectations(t)

	hostIDProvider.ShouldProvide("")

	storage := storage.NewSampler(ctx)
	m := NewSystemSampler(ctx, storage, nil, hostIDProvider)

	result, err := m.Sample()

	assert.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestSystemSample_HostIDMarshalling(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name         string
		sample       SystemSample
		expectedJSON string
	}{
		{
			name:         "this is the first case",
			sample:       SystemSample{HostID: "a host id"}, //nolint:exhaustruct
			expectedJSON: `{"eventType":"","timestamp":0,"entityKey":"","host.id":"a host id"}`,
		},
		{
			name:         "this is the first case",
			sample:       SystemSample{HostID: ""}, //nolint:exhaustruct
			expectedJSON: `{"eventType":"","timestamp":0,"entityKey":""}`,
		},
	}

	for i := range testCases {
		testCase := testCases[i]
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			res, err := json.Marshal(testCase.sample)
			require.NoError(t, err)
			assert.Equal(t, testCase.expectedJSON, string(res))
		})
	}
}

func TestSystemSample_HostID(t *testing.T) {
	t.Parallel()
	ctx := new(mocks.AgentContext)
	ctx.On("Config").Return(&config.Config{})

	hostIDProvider := &hostid.ProviderMock{}
	defer hostIDProvider.AssertExpectations(t)

	hostIDProvider.ShouldProvide("this-is-a-host-id")

	storage := storage.NewSampler(ctx)
	m := NewSystemSampler(ctx, storage, nil, hostIDProvider)

	result, err := m.Sample()

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "this-is-a-host-id", result[0].(*SystemSample).HostID) //nolint:forcetypeassert
}

func TestSystemSample_HostIDErrorShouldBeLogged(t *testing.T) {
	t.Parallel()
	ctx := new(mocks.AgentContext)
	ctx.On("Config").Return(&config.Config{})

	hostIDProvider := &hostid.ProviderMock{}
	defer hostIDProvider.AssertExpectations(t)

	hook := logHelper.NewInMemoryEntriesHook([]logrus.Level{logrus.ErrorLevel})
	log.AddHook(hook)

	//nolint:goerr113
	providerErr := errors.New("some error")
	hostIDProvider.ShouldReturnErr(providerErr)

	storage := storage.NewSampler(ctx)
	m := NewSystemSampler(ctx, storage, nil, hostIDProvider)

	result, err := m.Sample()

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "", result[0].(*SystemSample).HostID) //nolint:forcetypeassert
	assert.True(t, hook.EntryWithMessageExists(regexp.MustCompile(`cannot retrieve host_id`)))
	assert.True(t, hook.EntryWithErrorExists(providerErr))
}

func BenchmarkSystem(b *testing.B) {
	ctx := new(mocks.AgentContext)
	ctx.On("Config").Return(&config.Config{})

	hostIDProvider := &hostid.ProviderMock{}
	defer hostIDProvider.AssertExpectations(b)

	hostIDProvider.ShouldProvide("")

	storage := storage.NewSampler(ctx)
	m := NewSystemSampler(ctx, storage, nil, hostIDProvider)
	for n := 0; n < b.N; n++ {
		m.Sample()
	}
}
