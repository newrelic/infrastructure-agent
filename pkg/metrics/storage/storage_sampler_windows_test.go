// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package storage

import (
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWindowsPartitions(t *testing.T) {
	partitions, err := fetchPartitions(false)(false)
	assert.NoError(t, err)
	for _, partition := range partitions {
		if _, ok := SupportedFileSystems[partition.Fstype]; !ok {
			t.Fatalf("Unsupported file systems are in partitions, fs-type: %s", partition.Fstype)
		}
	}
}

func TestPdhIoCounters(t *testing.T) {
	sampler := NewStorageSampleWrapper(&config.Config{})

	counters, err := sampler.IOCounters()
	counters, err = sampler.IOCounters()
	require.NoError(t, err)
	assert.NotEmpty(t, counters)
}

func BenchmarkIoCounters_Pdh(b *testing.B) {
	s := NewStorageSampleWrapper(&config.Config{
		PartitionsTTL:        "60s",
		LegacyStorageSampler: false,
	})
	for n := 0; n < b.N; n++ {
		s.IOCounters()
	}
}

func BenchmarkIoCounters_Wmi(b *testing.B) {
	s := NewStorageSampleWrapper(&config.Config{
		PartitionsTTL:        "60s",
		LegacyStorageSampler: true,
	})
	for n := 0; n < b.N; n++ {
		s.IOCounters()
	}
}
