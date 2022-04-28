// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metrics

import (
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/storage"
	"github.com/stretchr/testify/assert"
)

func TestNewSystemSampler(t *testing.T) {
	cfg := config.Config{}
	cfg.MetricsSystemSampleRate = config.FREQ_INTERVAL_FLOOR_SYSTEM_METRICS
	cfg.IgnoreReclaimable = false
	cfg.Debug = false

	m := NewSystemSampler(nil, cfg.MetricsSystemSampleRate, cfg.IgnoreReclaimable, cfg.Debug)

	assert.NotNil(t, m)
}

func TestSystemSample(t *testing.T) {
	cfg := config.Config{}
	cfg.MetricsSystemSampleRate = config.FREQ_INTERVAL_FLOOR_SYSTEM_METRICS
	cfg.IgnoreReclaimable = false
	cfg.Debug = false

	s := storage.NewSampler(
		cfg.MetricsStorageSampleRate,
		cfg.PartitionsTTL,
		cfg.IsContainerized,
		cfg.WinRemovableDrives,
		cfg.CustomSupportedFileSystems,
		cfg.OverrideHostRoot,
	)
	m := NewSystemSampler(s, cfg.MetricsSystemSampleRate, cfg.IgnoreReclaimable, cfg.Debug)
	result, err := m.Sample()

	assert.NoError(t, err)
	assert.Len(t, result, 1)
}

func BenchmarkSystem(b *testing.B) {
	cfg := config.Config{}
	cfg.MetricsSystemSampleRate = config.FREQ_INTERVAL_FLOOR_SYSTEM_METRICS
	cfg.IgnoreReclaimable = false
	cfg.Debug = false

	s := storage.NewSampler(
		cfg.MetricsStorageSampleRate,
		cfg.PartitionsTTL,
		cfg.IsContainerized,
		cfg.WinRemovableDrives,
		cfg.CustomSupportedFileSystems,
		cfg.OverrideHostRoot,
	)
	m := NewSystemSampler(s, cfg.MetricsSystemSampleRate, cfg.IgnoreReclaimable, cfg.Debug)
	for n := 0; n < b.N; n++ {
		m.Sample()
	}
}
