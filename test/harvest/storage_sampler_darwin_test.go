// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build harvest || darwin
// +build harvest darwin

package harvest

import (
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/metrics/storage"
	"github.com/newrelic/infrastructure-agent/pkg/sample"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// func contextMock() *mocks.AgentContext {
// 	ctx := new(mocks.AgentContext)
// 	ctx.On("Config").Return(&config.Config{
// 		RunMode: config.ModeRoot,
// 		Log:     config.LogConfig{Level: config.LogLevelDebug},
// 	})
// 	ctx.On("GetServiceForPid", mock.Anything).Return("service-name", true)
// 	return ctx
// }

func TestStorageSample(t *testing.T) {
	du, err := disk.Usage("/")
	if err != nil {
		t.Skipf("this linux distro can't get the usage statistics: %v", err.Error())
	}
	if du.InodesTotal == 0 {
		t.Skipf("this linux distro is not supported for inodes: %#v", du)
	}

	// GIVEN a Storage Sampler
	ps := storage.NewSampler(contextMock())

	// THAT has already sampled values in the past
	_, err = ps.Sample()
	require.NoError(t, err)

	// WHEN it samples again
	samples, err := ps.Sample()
	require.NoError(t, err)

	// THEN the read samples are of the correct type, with a valid format and non-zero values for those
	// metrics that can't be zero
	ss := fullSample(t, samples)

	assert.Equal(t, "StorageSample", ss.EventType)

	assert.NotEmpty(t, ss.MountPoint)
	assert.NotEmpty(t, ss.Device)
	assert.NotEmpty(t, ss.FileSystemType)
	assert.NotEmpty(t, ss.Device)
	assert.NotEmpty(t, ss.IsReadOnly)
	assert.NotNil(t, ss.TotalBytes)
	assert.NotZero(t, *ss.TotalBytes)
	assert.NotNil(t, ss.UsedBytes)
	assert.NotZero(t, *ss.UsedBytes)
}

// fullSample returns a sample containing usage data
func fullSample(t *testing.T, samples sample.EventBatch) *storage.Sample {
	t.Helper()
	for i := range samples {
		if s, ok := samples[i].(*storage.Sample); ok {
			if s.UsedBytes != nil {
				return s
			}
		}
	}
	require.Failf(t, "can't find valid storage sample", "%#v", samples)
	return nil
}
