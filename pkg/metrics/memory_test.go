// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metrics

import (
	"github.com/shirou/gopsutil/mem"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryMonitor_Sample(t *testing.T) {
	m := NewMemoryMonitor(false)

	sample, err := m.Sample()
	require.NoError(t, err)

	assert.NotZero(t, sample.MemoryTotal)
	assert.NotZero(t, sample.MemoryFree)
	assert.NotZero(t, sample.MemoryUsed)
	assert.NotZero(t, sample.MemoryFreePercent)
	assert.NotZero(t, sample.MemoryUsedPercent)
	assert.InDelta(t, sample.MemoryTotal, sample.MemoryFree+sample.MemoryUsed, 0.1)

	assert.InDelta(t, sample.SwapTotal, sample.SwapFree+sample.SwapUsed, 0.1)
}

func TestMemoryMonitor_Sample_CheckFreeAndUsedPercentage(t *testing.T) {
	vms := &mem.VirtualMemoryStat{
		Total:     10000,
		Available: 7500,
		Used:      2500,
	}

	m := &MemoryMonitor{vmHarvest: func() (*mem.VirtualMemoryStat, error) {
		return vms, nil
	}}

	sample, err := m.Sample()
	require.NoError(t, err)

	assert.EqualValues(t, vms.Total, sample.MemoryTotal)
	assert.EqualValues(t, vms.Available, sample.MemoryFree)
	assert.EqualValues(t, vms.Used, sample.MemoryUsed)
	assert.EqualValues(t, float64(vms.Available)/float64(vms.Total)*100, sample.MemoryFreePercent)
	assert.EqualValues(t, float64(vms.Used)/float64(vms.Total)*100, sample.MemoryUsedPercent)
}
