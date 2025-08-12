// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metrics

import (
	"encoding/json"
	"runtime"
	"testing"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// Operating system constants for testing.
	windowsOS = "windows"
)

func TestNewCPUMonitor(t *testing.T) {
	t.Parallel()

	monitor := NewCPUMonitor(nil)

	assert.NotNil(t, monitor)

	// Test platform-specific initialization
	if runtime.GOOS == windowsOS {
		assert.NotNil(t, monitor.windowsMonitor, "Windows monitor should be initialized")
		assert.Nil(t, monitor.cpuTimes, "cpuTimes function should be nil on Windows")
	} else {
		assert.Nil(t, monitor.windowsMonitor, "Windows monitor should be nil on non-Windows platforms")
		assert.NotNil(t, monitor.cpuTimes, "cpuTimes function should be set on non-Windows platforms")
	}
}

func TestCPUSample(t *testing.T) {
	t.Parallel()

	monitor := NewCPUMonitor(nil)

	result, err := monitor.Sample()

	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Test that CPU percentages are within valid ranges
	assert.GreaterOrEqual(t, result.CPUPercent, 0.0, "CPU percent should be >= 0")
	assert.LessOrEqual(t, result.CPUPercent, 100.0, "CPU percent should be <= 100")
	assert.GreaterOrEqual(t, result.CPUUserPercent, 0.0, "CPU user percent should be >= 0")
	assert.LessOrEqual(t, result.CPUUserPercent, 100.0, "CPU user percent should be <= 100")
	assert.GreaterOrEqual(t, result.CPUSystemPercent, 0.0, "CPU system percent should be >= 0")
	assert.LessOrEqual(t, result.CPUSystemPercent, 100.0, "CPU system percent should be <= 100")
	assert.GreaterOrEqual(t, result.CPUIdlePercent, 0.0, "CPU idle percent should be >= 0")
	assert.LessOrEqual(t, result.CPUIdlePercent, 100.0, "CPU idle percent should be <= 100")

	// Platform-specific validations
	if runtime.GOOS == windowsOS {
		// Windows doesn't report IOWait or Steal
		assert.InDelta(t, 0.0, result.CPUIOWaitPercent, 0.001, "Windows should not report IOWait")
		assert.InDelta(t, 0.0, result.CPUStealPercent, 0.001, "Windows should not report Steal time")
	}
}

func TestCPUSample_MultipleCalls(t *testing.T) {
	t.Parallel()

	monitor := NewCPUMonitor(nil)

	// First call might return empty sample on non-Windows platforms
	result1, err := monitor.Sample()
	require.NoError(t, err)
	assert.NotNil(t, result1)

	// Second call should return meaningful data
	result2, err := monitor.Sample()
	require.NoError(t, err)
	assert.NotNil(t, result2)

	// On non-Windows platforms, first call might be empty, second should have data
	if runtime.GOOS != windowsOS && result1.CPUPercent == 0 {
		// This is expected behavior for the first sample
		assert.GreaterOrEqual(t, result2.CPUPercent, 0.0)
	}
}

func TestCPUSample_JSON_Marshaling(t *testing.T) {
	t.Parallel()

	monitor := NewCPUMonitor(nil)

	sample, err := monitor.Sample()
	require.NoError(t, err)

	// Test JSON marshaling
	jsonData, err := json.Marshal(sample)
	require.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	// Test JSON unmarshaling
	var unmarshaledSample CPUSample
	err = json.Unmarshal(jsonData, &unmarshaledSample)
	require.NoError(t, err)
	assert.InDelta(t, sample.CPUPercent, unmarshaledSample.CPUPercent, 0.001)
	assert.InDelta(t, sample.CPUUserPercent, unmarshaledSample.CPUUserPercent, 0.001)
	assert.InDelta(t, sample.CPUSystemPercent, unmarshaledSample.CPUSystemPercent, 0.001)
	assert.InDelta(t, sample.CPUIOWaitPercent, unmarshaledSample.CPUIOWaitPercent, 0.001)
	assert.InDelta(t, sample.CPUIdlePercent, unmarshaledSample.CPUIdlePercent, 0.001)
	assert.InDelta(t, sample.CPUStealPercent, unmarshaledSample.CPUStealPercent, 0.001)
}

func TestCPUMonitor_Close(t *testing.T) {
	t.Parallel()

	monitor := NewCPUMonitor(nil)

	// Test that Close() doesn't return an error
	err := monitor.Close()
	require.NoError(t, err)

	// Test that Close() can be called multiple times
	err = monitor.Close()
	assert.NoError(t, err)
}

// Tests for non-Windows platforms (Linux/Darwin).
func TestCpuMarshallableSample_NormalOperation(t *testing.T) {
	if runtime.GOOS == windowsOS {
		t.Skip("This test is for non-Windows platforms")
	}

	cpuTimes := func(_ bool) ([]cpu.TimesStat, error) {
		return []cpu.TimesStat{{
			CPU: "1", User: 1.0, Nice: 1.0, System: 1.0, Idle: 1.0,
			Iowait: 1.0, Irq: 1.0, Softirq: 1.0, Steal: 1.0, Guest: 1.0, GuestNice: 1.0,
		}}, nil
	}
	cpuMon := CPUMonitor{
		context:  nil,
		cpuTimes: cpuTimes,
		last: []cpu.TimesStat{{
			CPU: "1", User: 0.0, Nice: 0.0, System: 0.0, Idle: 0.0,
			Iowait: 0.0, Irq: 0.0, Softirq: 0.0, Steal: 0.0, Guest: 0.0, GuestNice: 0.0,
		}},
		windowsMonitor: nil,
	}
	sample, err := cpuMon.Sample()
	assert.NoError(t, err)
	_, err = json.Marshal(sample)
	assert.NoError(t, err)
}

func TestCpuMarshallableSample_ZeroDeltas(t *testing.T) {
	if runtime.GOOS == windowsOS {
		t.Skip("This test is for non-Windows platforms")
	}

	cpuTimes := func(_ bool) ([]cpu.TimesStat, error) {
		return []cpu.TimesStat{{
			CPU: "1", User: 1.0, Nice: 1.0, System: 1.0, Idle: 1.0,
			Iowait: 1.0, Irq: 1.0, Softirq: 1.0, Steal: 1.0, Guest: 1.0, GuestNice: 1.0,
		}}, nil
	}
	cpuMon := CPUMonitor{
		context:  nil,
		cpuTimes: cpuTimes,
		last: []cpu.TimesStat{{
			CPU: "1", User: 1.0, Nice: 1.0, System: 1.0, Idle: 1.0,
			Iowait: 1.0, Irq: 1.0, Softirq: 1.0, Steal: 1.0, Guest: 1.0, GuestNice: 1.0,
		}},
		windowsMonitor: nil,
	}
	sample, err := cpuMon.Sample()
	assert.NoError(t, err)
	_, err = json.Marshal(sample)
	assert.NoError(t, err)
}

func TestCPUSample_EmptyCurrentTimes(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == windowsOS {
		t.Skip("This test is for non-Windows platforms")
	}

	// Test case for empty current times (container environments)
	cpuTimes := func(_ bool) ([]cpu.TimesStat, error) {
		return []cpu.TimesStat{}, nil
	}
	cpuMon := CPUMonitor{
		context:  nil,
		cpuTimes: cpuTimes,
		last: []cpu.TimesStat{{
			CPU: "1", User: 1.0, Nice: 1.0, System: 1.0, Idle: 1.0,
			Iowait: 1.0, Irq: 1.0, Softirq: 1.0, Steal: 1.0, Guest: 1.0, GuestNice: 1.0,
		}},
		windowsMonitor: nil,
	}
	sample, err := cpuMon.Sample()
	require.NoError(t, err)
	assert.NotNil(t, sample)
	// Should return empty sample without panic
	assert.InDelta(t, 0.0, sample.CPUPercent, 0.001)
}

func TestCPUDelta(t *testing.T) {
	t.Parallel()

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

	result := cpuDelta(cpu1, cpu2)
	expected := &cpu.TimesStat{
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
	}
	assert.Equal(t, expected, result)
}

func TestCPUDelta_NegativeSteal(t *testing.T) {
	t.Parallel()

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

	result := cpuDelta(current, previous)
	expected := &cpu.TimesStat{
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
	}
	assert.Equal(t, expected, result)
}
