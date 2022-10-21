// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"math"
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

	if runtime.GOOS != "windows" {
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
	fs := map[string]string{
		"linux":   "xfs",
		"windows": "NTFS",
		"darwin":  "apfs",
	}

	oldSupportedFileSystems := SupportedFileSystems
	defer func() {
		SupportedFileSystems = oldSupportedFileSystems
	}()
	cfg := config.Config{CustomSupportedFileSystems: []string{fs[runtime.GOOS]},
		FileDevicesIgnored: []string{"sda1"}, MetricsStorageSampleRate: config.FREQ_INTERVAL_FLOOR_STORAGE_METRICS}

	testAgent, err := agent.NewAgent(
		&cfg,
		"1",
		"userAgent",
		test.EmptyFFRetriever)
	assert.NoError(t, err)
	testAgentConfig := testAgent.Context

	m := NewSampler(testAgentConfig)
	testSampleQueue := make(chan sample.EventBatch, 2)
	metrics.StartSamplerRoutine(m, testSampleQueue)
	assert.NoError(t, err)
	time.Sleep(1 * time.Second)
	assert.Len(t, SupportedFileSystems, 1)
	assert.Contains(t, SupportedFileSystems, fs[runtime.GOOS])
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
		_, _ = m.Sample()
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

func TestAsValidFloatPtr(t *testing.T) {
	toPtr := func(f float64) *float64 {
		return &f
	}
	var floatPtrTest = []struct {
		name     string
		in       *float64
		outIsNil bool
	}{
		{"Positive float value", toPtr(3.0), false},
		{"Negative float value", toPtr(-3.0), false},
		{"NaN float", toPtr(math.NaN()), true},
		{"Infinite float", toPtr(math.Inf(0)), true},
		{"Nil pointer", nil, true},
	}

	for _, tt := range floatPtrTest {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.outIsNil, asValidFloatPtr(tt.in) == nil)
		})
	}
}

func TestPopulatePartition(t *testing.T) {

	var partitionTests = []struct {
		name      string
		partition PartitionStat
		expected  Sample
	}{
		{
			"rw_ext4",
			PartitionStat{
				Device:     "/dev/sda",
				Mountpoint: "/",
				Fstype:     "ext4",
				Opts:       "rw",
			},
			Sample{
				BaseSample: BaseSample{
					MountPoint:     "/",
					Device:         "/dev/sda",
					IsReadOnly:     "false",
					FileSystemType: "ext4",
				},
			},
		},
		{
			"ro_ext2",
			PartitionStat{
				Device:     "/dev/sda",
				Mountpoint: "/",
				Fstype:     "ext2",
				Opts:       "ro",
			},
			Sample{
				BaseSample: BaseSample{
					MountPoint:     "/",
					Device:         "/dev/sda",
					IsReadOnly:     "true",
					FileSystemType: "ext2",
				},
			},
		},
		{
			"rwro_ext3",
			PartitionStat{
				Device:     "/dev/sda",
				Mountpoint: "/",
				Fstype:     "ext3",
				Opts:       "rw,ro",
			},
			Sample{
				BaseSample: BaseSample{
					MountPoint:     "/",
					Device:         "/dev/sda",
					IsReadOnly:     "true",
					FileSystemType: "ext3",
				},
			},
		},
		{
			"rorw_ext3",
			PartitionStat{
				Device:     "/dev/sda",
				Mountpoint: "/",
				Fstype:     "ext3",
				Opts:       "ro,rw",
			},
			Sample{
				BaseSample: BaseSample{
					MountPoint:     "/",
					Device:         "/dev/sda",
					IsReadOnly:     "true",
					FileSystemType: "ext3",
				},
			},
		},
	}

	for _, tt := range partitionTests {
		t.Run(tt.name, func(t *testing.T) {
			actualSample := Sample{}
			populatePartition(tt.partition, &actualSample)
			assert.Equal(t, tt.expected, actualSample)
		})
	}
}
