// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux windows

package storage

import (
	"runtime"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent/mocks"
	"github.com/newrelic/infrastructure-agent/internal/feature_flags/test"
	"github.com/newrelic/infrastructure-agent/pkg/sample"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	metrics "github.com/newrelic/infrastructure-agent/pkg/metrics/sampler"
)

func TestNewStorageSampler(t *testing.T) {
	ctx := new(mocks.AgentContext)
	ctx.On("Config").Return(&config.Config{})

	m := NewSampler(ctx)

	assert.NotNil(t, m)
}

func TestStorageSample(t *testing.T) {
	ctx := new(mocks.AgentContext)
	ctx.On("Config").Return(&config.Config{})

	m := NewSampler(ctx)

	result, err := m.Sample()

	assert.NoError(t, err)

	if runtime.GOOS == "linux" {
		if len(result) > 0 {
			// succeed
		} else {
			t.Fatal("StorageSampler couldn't find any filesystems on Unix system?")
		}
	} else {
		t.Skip("Unsupported platform for StorageSampler")
	}
}

func TestSampleWithCustomFilesystemList(t *testing.T) {
	fs := "xfs"
	if runtime.GOOS == "windows" {
		fs = "NTFS"
	}

	oldSupportedFileSystems := SupportedFileSystems
	defer func() {
		SupportedFileSystems = oldSupportedFileSystems
	}()
	config := config.Config{CustomSupportedFileSystems: []string{fs},
		FileDevicesIgnored: []string{"sda1"}, MetricsStorageSampleRate: config.FREQ_INTERVAL_FLOOR_STORAGE_METRICS}

	testAgent, err := agent.NewAgent(&config, "1", test.EmptyFFRetriever)
	assert.NoError(t, err)
	testAgentConfig := testAgent.Context

	m := NewSampler(testAgentConfig)
	testSampleQueue := make(chan sample.EventBatch, 2)
	metrics.StartSamplerRoutine(m, testSampleQueue)
	assert.NoError(t, err)
	time.Sleep(1 * time.Second)
	assert.Contains(t, SupportedFileSystems, fs)
}

func TestPartitionsCache(t *testing.T) {
	// Given a partitions cache
	pc := PartitionsCache{
		ttl: time.Hour,
		partitionsFunc: func(_ bool) ([]PartitionStat, error) {
			return make([]PartitionStat, 2), nil
		},
	}
	// When invoked for a first time
	partitions, _ := pc.Get()
	// It returns the discovered partitions
	assert.Len(t, partitions, 2)

	// And when the partitions change before the TTL period
	pc.partitionsFunc = func(_ bool) ([]PartitionStat, error) {
		return make([]PartitionStat, 3), nil
	}
	// The cache still returns the old value
	partitions, _ = pc.Get()
	assert.Len(t, partitions, 2)

	// Until the TTL is reached
	pc.lastInvocation = pc.lastInvocation.Add(-2 * time.Hour)
	partitions, _ = pc.Get()
	assert.Len(t, partitions, 3)
}

func TestPartitionsCache_Error(t *testing.T) {
	// Given a partitions cache
	pc := PartitionsCache{
		ttl: time.Hour,
		partitionsFunc: func(_ bool) ([]PartitionStat, error) {
			return nil, errors.New("patapun")
		},
	}
	// When there is an error returning the partitions
	_, err := pc.Get()
	// The cache returns the original error
	assert.EqualError(t, err, "patapun")
}

func BenchmarkStorage(b *testing.B) {
	ctx := new(mocks.AgentContext)
	ctx.On("Config").Return(&config.Config{})

	m := NewSampler(ctx)

	for n := 0; n < b.N; n++ {
		m.Sample()
	}
}

func TestCalculateReadWriteBytesPerSecond(t *testing.T) {

	var f64toPointer = func(variable float64) *float64 {
		return &variable
	}

	testCases := []struct {
		read     *float64
		write    *float64
		expected *float64
	}{
		{
			read:     f64toPointer(13),
			write:    f64toPointer(29),
			expected: f64toPointer(42),
		},
		{
			read:     nil,
			write:    f64toPointer(29),
			expected: nil,
		},
		{
			read:     f64toPointer(13),
			write:    nil,
			expected: nil,
		},
		{
			read:     nil,
			write:    nil,
			expected: nil,
		},
	}

	for _, testCase := range testCases {
		actual := calculateReadWriteBytesPerSecond(testCase.read, testCase.write)
		assert.Equal(t, testCase.expected, actual)
	}
}
