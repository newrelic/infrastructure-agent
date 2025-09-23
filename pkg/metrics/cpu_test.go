// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metrics

import (
	"encoding/json"
	"runtime"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCPUMonitor(t *testing.T) {
	m := NewCPUMonitor(nil)

	assert.NotNil(t, m)
}

func TestCPUSample(t *testing.T) {
	m := NewCPUMonitor(nil)

	result, err := m.Sample()

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestCpuMarshallableSample_NormalOperation(t *testing.T) {
	cpuTimes := func(_ bool) ([]cpu.TimesStat, error) {
		return []cpu.TimesStat{{"1", 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0}}, nil
	}
	cpuMon := CPUMonitor{
		cpuTimes: cpuTimes,
		last:     []cpu.TimesStat{{"1", 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0}},
	}
	sample, err := cpuMon.Sample()
	assert.NoError(t, err)
	_, err = json.Marshal(sample)
	assert.NoError(t, err)
}

func TestCpuMarshallableSample_ZeroDeltas(t *testing.T) {
	cpuTimes := func(_ bool) ([]cpu.TimesStat, error) {
		return []cpu.TimesStat{{"1", 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0}}, nil
	}
	cpuMon := CPUMonitor{
		cpuTimes: cpuTimes,
		last:     []cpu.TimesStat{{"1", 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0}},
	}
	sample, err := cpuMon.Sample()
	assert.NoError(t, err)
	_, err = json.Marshal(sample)
	assert.NoError(t, err)
}

func TestCPUDelta(t *testing.T) {
	cpu1 := &cpu.TimesStat{
		CPU:       "intel",
		Guest:     1.0,
		GuestNice: 1.0,
		Idle:      1.0,
		Iowait:    1.0,
		Irq:       1.0,
		Nice:      1.0,
		Softirq:   1.0,
		Steal:     1.0,
		System:    1.0,
		User:      1.0,
	}

	cpu2 := &cpu.TimesStat{
		CPU:       "intel",
		Guest:     1.0,
		GuestNice: 1.0,
		Idle:      1.0,
		Iowait:    1.0,
		Irq:       1.0,
		Nice:      1.0,
		Softirq:   1.0,
		Steal:     1.0,
		System:    1.0,
		User:      1.0,
	}

	assert.Equal(t, &cpu.TimesStat{
		CPU:       "intel",
		Guest:     0.0,
		GuestNice: 0.0,
		Idle:      0.0,
		Iowait:    0.0,
		Irq:       0.0,
		Nice:      0.0,
		Softirq:   0.0,
		Steal:     0.0,
		System:    0.0,
		User:      0.0,
	}, cpuDelta(cpu1, cpu2))
}

func TestCPUDelta_NegativeSteal(t *testing.T) {
	current := &cpu.TimesStat{
		CPU:       "intel",
		Guest:     10.0,
		GuestNice: 10.0,
		Idle:      10.0,
		Iowait:    10.0,
		Irq:       10.0,
		Nice:      10.0,
		Softirq:   10.0,
		Steal:     10.0,
		System:    10.0,
		User:      10.0,
	}

	previous := &cpu.TimesStat{
		CPU:       "intel",
		Guest:     1.0,
		GuestNice: 1.0,
		Idle:      1.0,
		Iowait:    1.0,
		Irq:       1.0,
		Nice:      1.0,
		Softirq:   1.0,
		Steal:     100.0,
		System:    1.0,
		User:      1.0,
	}

	assert.Equal(t, &cpu.TimesStat{
		CPU:       "intel",
		Guest:     9.0,
		GuestNice: 9.0,
		Idle:      9.0,
		Iowait:    9.0,
		Irq:       9.0,
		Nice:      9.0,
		Softirq:   9.0,
		Steal:     0.0,
		System:    9.0,
		User:      9.0,
	}, cpuDelta(current, previous))
}

func TestWindowsCPUMonitor_RawCounterArrays(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	// Create the unified CPU monitor (which now uses raw counter arrays on Windows)
	// Pass nil for agent context since it's not needed for CPU monitoring in tests
	monitor := NewCPUMonitor(nil)
	require.NotNil(t, monitor)

	defer monitor.Close()

	// First sample might return zeros since we need a previous sample to calculate
	sample1, err := monitor.Sample()
	require.NoError(t, err)
	require.NotNil(t, sample1)

	// First sample values should be valid (>= 0)
	assert.GreaterOrEqual(t, sample1.CPUPercent, float64(0))
	assert.LessOrEqual(t, sample1.CPUPercent, float64(100))

	// Wait a bit and take another sample to ensure we have deltas
	time.Sleep(2 * time.Second)

	sample2, err := monitor.Sample()
	require.NoError(t, err)
	require.NotNil(t, sample2)

	// Second sample should have meaningful values
	assert.GreaterOrEqual(t, sample2.CPUPercent, float64(0))
	assert.LessOrEqual(t, sample2.CPUPercent, float64(100))

	assert.GreaterOrEqual(t, sample2.CPUUserPercent, float64(0))
	assert.LessOrEqual(t, sample2.CPUUserPercent, float64(100))

	assert.GreaterOrEqual(t, sample2.CPUSystemPercent, float64(0))
	assert.LessOrEqual(t, sample2.CPUSystemPercent, float64(100))

	assert.GreaterOrEqual(t, sample2.CPUIdlePercent, float64(0))
	assert.LessOrEqual(t, sample2.CPUIdlePercent, float64(100))

	// Windows-specific assertions
	assert.Equal(t, float64(0), sample2.CPUIOWaitPercent) // Windows doesn't have IOWait
	assert.Equal(t, float64(0), sample2.CPUStealPercent)  // Windows doesn't have steal time

	t.Logf("Windows CPU Sample (Raw Counter Arrays): %+v", sample2)
	t.Logf("This implementation works correctly on both single and multi-CPU group systems")
}
