// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build windows && harvest
// +build windows,harvest

package harvest

import (
	"github.com/newrelic/infrastructure-agent/internal/agent/mocks"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/storage"
	"github.com/newrelic/infrastructure-agent/pkg/sample"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestStorageSample(t *testing.T) {
	// GIVEN a Storage Sampler
	ps := storage.NewSampler(contextMock())

	// THAT has already sampled values in the past
	_, err := ps.Sample()
	require.NoError(t, err)

	// WHEN it samples again
	samples, err := ps.Sample()
	require.NoError(t, err)

	// THEN the read samples are of the correct type, with a valid format and non-zero values for those
	// metrics that can't be zero
	ss := rootSample(t, samples)

	assert.Equal(t, "StorageSample", ss.EventType)

	assert.NotEmpty(t, ss.MountPoint)
	assert.NotEmpty(t, ss.Device)
	assert.NotEmpty(t, ss.FileSystemType)
	assert.NotEmpty(t, ss.Device)
	assert.NotEmpty(t, ss.IsReadOnly)

	assert.NotNil(t, ss.TotalUtilizationPercent)
	assert.NotNil(t, ss.ReadUtilizationPercent)
	assert.NotNil(t, ss.WriteUtilizationPercent)
	assert.NotNil(t, ss.ReadsPerSec)
	assert.NotNil(t, ss.WritesPerSec)
	assert.NotNil(t, ss.ReadBytesPerSec)
	assert.NotNil(t, ss.WriteBytesPerSec)
	assert.NotNil(t, ss.IOTimeDelta)
	assert.NotNil(t, ss.ReadTimeDelta)
	assert.NotNil(t, ss.WriteTimeDelta)
	assert.NotNil(t, ss.ReadCountDelta)
	assert.NotNil(t, ss.WriteCountDelta)

	assert.NotNil(t, ss.AvgQueueLen)
	assert.NotNil(t, ss.AvgReadQueueLen)
	assert.NotNil(t, ss.AvgWriteQueueLen)
	assert.NotNil(t, ss.CurrentQueueLen)
}

func contextMock() *mocks.AgentContext {
	ctx := new(mocks.AgentContext)
	ctx.On("Config").Return(&config.Config{
		RunMode: config.ModeRoot,
		Log:     config.LogConfig{Level: config.LogLevelDebug},
	})
	ctx.On("GetServiceForPid", mock.Anything).Return("service-name", true)
	return ctx
}

func rootSample(t *testing.T, samples sample.EventBatch) *storage.Sample {
	t.Helper()
	for _, sample := range samples {
		if s, ok := sample.(*storage.Sample); ok {
			if s.MountPoint == "C:" {
				return s
			}
		}
	}
	t.Errorf("can't find root storage sample: %#v", samples)
	return nil
}
