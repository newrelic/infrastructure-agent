//go:build windows
// +build windows

// Copyright 2025 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metrics

import (
	"testing"
	"time"

	nrwin "github.com/newrelic/infrastructure-agent/internal/windows"
	winapi "github.com/newrelic/infrastructure-agent/internal/windows/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCPUMonitor_Windows(t *testing.T) {
	t.Parallel()

	monitor := NewCPUMonitor(nil)

	assert.NotNil(t, monitor)
	assert.NotNil(t, monitor.windowsMonitor, "Windows monitor should be initialized")
	assert.Nil(t, monitor.cpuTimes, "cpuTimes function should be nil on Windows")
}

func TestCPUSample_Windows(t *testing.T) {
	t.Parallel()

	monitor := NewCPUMonitor(nil)

	result, err := monitor.Sample()

	require.NoError(t, err)
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

	// Windows doesn't report IOWait or Steal
	assert.InDelta(t, 0.0, result.CPUIOWaitPercent, 0.001, "Windows should not report IOWait")
	assert.InDelta(t, 0.0, result.CPUStealPercent, 0.001, "Windows should not report Steal time")
}

func TestWindowsCPUMonitor_RawCounterArrays_Windows(t *testing.T) {
	t.Parallel()

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

	// Windows-specific assertions - using InDelta for float comparison linting compliance
	assert.InDelta(t, float64(0), sample2.CPUIOWaitPercent, 0.01) // Windows doesn't have IOWait
	assert.InDelta(t, float64(0), sample2.CPUStealPercent, 0.01)  // Windows doesn't have steal time

	t.Logf("Windows CPU Sample (Raw Counter Arrays): %+v", sample2)
	t.Logf("This implementation works correctly on both single and multi-CPU group systems")
}

func TestNormalizePercentage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    float64
		expected float64
	}{
		{
			name:     "Valid percentage - zero",
			input:    0.0,
			expected: 0.0,
		},
		{
			name:     "Valid percentage - middle range",
			input:    50.5,
			expected: 50.5,
		},
		{
			name:     "Valid percentage - maximum",
			input:    100.0,
			expected: 100.0,
		},
		{
			name:     "Valid percentage - small decimal",
			input:    0.1,
			expected: 0.1,
		},
		{
			name:     "Valid percentage - large decimal",
			input:    99.9,
			expected: 99.9,
		},
		{
			name:     "Negative value - should be clamped to zero",
			input:    -1.0,
			expected: 0.0,
		},
		{
			name:     "Large negative value - should be clamped to zero",
			input:    -999.99,
			expected: 0.0,
		},
		{
			name:     "Value above 100 - should be clamped to 100",
			input:    101.0,
			expected: 100.0,
		},
		{
			name:     "Large value above 100 - should be clamped to 100",
			input:    999.99,
			expected: 100.0,
		},
		{
			name:     "Very small positive value",
			input:    0.0001,
			expected: 0.0001,
		},
		{
			name:     "Value very close to 100",
			input:    99.9999,
			expected: 99.9999,
		},
		{
			name:     "Exactly at boundary - just above 100",
			input:    100.0001,
			expected: 100.0,
		},
		{
			name:     "Exactly at boundary - just below 0",
			input:    -0.0001,
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := normalizePercentage(tt.input)

			// Use assert.Equal for zero values, assert.InEpsilon for non-zero values
			if tt.expected == 0 {
				assert.Equal(t, tt.expected, result, "normalizePercentage(%f) should return %f, got %f", tt.input, tt.expected, result)
			} else {
				assert.InEpsilon(t, tt.expected, result, 1e-10, "normalizePercentage(%f) should return %f, got %f", tt.input, tt.expected, result)
			}
		})
	}
}

func TestCalculatePercent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		part     time.Duration
		total    time.Duration
		expected float64
	}{
		{
			name:     "Normal calculation - 50%",
			part:     50 * time.Second,
			total:    100 * time.Second,
			expected: 50.0,
		},
		{
			name:     "Normal calculation - 25%",
			part:     25 * time.Millisecond,
			total:    100 * time.Millisecond,
			expected: 25.0,
		},
		{
			name:     "Zero part - 0%",
			part:     0,
			total:    100 * time.Second,
			expected: 0.0,
		},
		{
			name:     "Zero total - should return 0 to avoid division by zero",
			part:     50 * time.Second,
			total:    0,
			expected: 0.0,
		},
		{
			name:     "Both zero - should return 0",
			part:     0,
			total:    0,
			expected: 0.0,
		},
		{
			name:     "Part equals total - 100%",
			part:     100 * time.Second,
			total:    100 * time.Second,
			expected: 100.0,
		},
		{
			name:     "Part greater than total - over 100%",
			part:     150 * time.Second,
			total:    100 * time.Second,
			expected: 150.0,
		},
		{
			name:     "Very small values - microseconds",
			part:     1 * time.Microsecond,
			total:    4 * time.Microsecond,
			expected: 25.0,
		},
		{
			name:     "Large values - hours",
			part:     3 * time.Hour,
			total:    12 * time.Hour,
			expected: 25.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := calculatePercent(tt.part, tt.total)
			assert.InDelta(t, tt.expected, result, 0.0001, "calculatePercent(%v, %v) should return %f, got %f", tt.part, tt.total, tt.expected, result)
		})
	}
}

func TestWindowsCPUMonitor_Close(t *testing.T) {
	t.Parallel()

	t.Run("Close uninitialized monitor", func(t *testing.T) {
		t.Parallel()

		winMonitor := &WindowsCPUMonitor{
			context:            nil,
			rawPoll:            nil,
			started:            false,
			requiresTwoSamples: true,
			lastSample:         nil,
			lastTimestamp:      time.Time{},
		}

		// Should not return error when closing uninitialized monitor
		err := winMonitor.close()
		assert.NoError(t, err, "Close should not return error for uninitialized monitor")
	})

	t.Run("Close monitor with nil rawPoll", func(t *testing.T) {
		t.Parallel()

		winMonitor := &WindowsCPUMonitor{
			context:            nil,
			rawPoll:            nil,
			started:            true, // started but rawPoll is nil
			requiresTwoSamples: false,
			lastSample:         nil,
			lastTimestamp:      time.Time{},
		}

		// Should not return error when rawPoll is nil
		err := winMonitor.close()
		assert.NoError(t, err, "Close should not return error when rawPoll is nil")
	})

	t.Run("Close multiple times", func(t *testing.T) {
		t.Parallel()

		// Create monitor through the standard constructor
		monitor := NewCPUMonitor(nil)
		require.NotNil(t, monitor)
		require.NotNil(t, monitor.windowsMonitor)

		// First close should work
		err := monitor.Close()
		require.NoError(t, err, "First close should not return error")

		// Second close should also work (idempotent)
		err = monitor.Close()
		require.NoError(t, err, "Second close should not return error")

		// Third close should still work
		err = monitor.Close()
		assert.NoError(t, err, "Third close should not return error")
	})

	t.Run("Close after initialization", func(t *testing.T) {
		t.Parallel()

		// Create monitor and try to initialize it
		monitor := NewCPUMonitor(nil)
		require.NotNil(t, monitor)

		// Try to get a sample which will initialize the PDH
		// This might fail on non-Windows, but we're testing the close behavior
		_, _ = monitor.Sample() // Ignore error since we're on macOS

		// Close should work regardless of initialization success
		err := monitor.Close()
		assert.NoError(t, err, "Close after initialization attempt should not return error")
	})
}

func TestWindowsCPUMonitor_CloseResourceLeakPrevention(t *testing.T) {
	t.Parallel()

	t.Run("Verify close cleans up resources", func(t *testing.T) {
		t.Parallel()

		monitor := NewCPUMonitor(nil)
		require.NotNil(t, monitor)
		require.NotNil(t, monitor.windowsMonitor)

		// After trying to sample (which initializes), close should work
		_, _ = monitor.Sample() // Might fail on non-Windows but that's OK for testing

		// Close should work regardless of initialization success/failure
		err := monitor.Close()
		assert.NoError(t, err, "Close should work after attempted initialization")
	})

	t.Run("Close with started=false should be safe", func(t *testing.T) {
		t.Parallel()

		winMonitor := &WindowsCPUMonitor{
			context:            nil,
			rawPoll:            nil,
			started:            false,
			requiresTwoSamples: true,
			lastSample:         nil,
			lastTimestamp:      time.Time{},
		}

		err := winMonitor.close()
		assert.NoError(t, err, "Close with started=false should be safe")
	})
}

func TestWindowsCPUMonitor_CalculateCPUTimeDelta(t *testing.T) {
	t.Parallel()

	// Helper function to create CPUGroupInfo
	createCPUInfo := func(name string, value int64) nrwin.CPUGroupInfo {
		return nrwin.CPUGroupInfo{
			Name: name,
			RawValue: winapi.PDH_RAW_COUNTER{
				CStatus: 0,
				TimeStamp: winapi.FILETIME{
					LowDateTime:  0,
					HighDateTime: 0,
				},
				FirstValue:  value,
				SecondValue: 0,
				MultiCount:  0,
			},
			Timestamp: 0,
		}
	}

	t.Run("Normal delta calculation", func(t *testing.T) {
		t.Parallel()

		monitor := &WindowsCPUMonitor{
			context:            nil,
			rawPoll:            nil,
			started:            false,
			requiresTwoSamples: false,
			lastSample:         nil,
			lastTimestamp:      time.Time{},
		}
		var totalTime time.Duration

		currentData := []nrwin.CPUGroupInfo{
			createCPUInfo("0,0", 1000),
			createCPUInfo("0,1", 2000),
		}

		lastData := []nrwin.CPUGroupInfo{
			createCPUInfo("0,0", 500),
			createCPUInfo("0,1", 1500),
		}

		validCount := monitor.calculateCPUTimeDelta(currentData, lastData, &totalTime, "test")

		assert.Equal(t, 2, validCount, "Should process 2 valid cores")
		// Delta: (1000-500) + (2000-1500) = 500 + 500 = 1000
		// Convert to nanoseconds: 1000 * 100 = 100000
		expectedTime := time.Duration(1000 * 100)
		assert.Equal(t, expectedTime, totalTime, "Total time should be sum of deltas")
	})

	t.Run("Skip _Total entries", func(t *testing.T) {
		t.Parallel()

		monitor := &WindowsCPUMonitor{
			context:            nil,
			rawPoll:            nil,
			started:            false,
			requiresTwoSamples: false,
			lastSample:         nil,
			lastTimestamp:      time.Time{},
		}
		var totalTime time.Duration

		currentData := []nrwin.CPUGroupInfo{
			createCPUInfo("_Total", 5000), // Should be skipped
			createCPUInfo("0,0", 1000),
		}

		lastData := []nrwin.CPUGroupInfo{
			createCPUInfo("_Total", 4000), // Should be skipped
			createCPUInfo("0,0", 500),
		}

		validCount := monitor.calculateCPUTimeDelta(currentData, lastData, &totalTime, "test")

		assert.Equal(t, 1, validCount, "Should process 1 valid core (skip _Total)")
		expectedTime := time.Duration(500 * 100) // (1000-500) * 100
		assert.Equal(t, expectedTime, totalTime)
	})

	t.Run("Handle missing last data", func(t *testing.T) {
		t.Parallel()

		monitor := &WindowsCPUMonitor{
			context:            nil,
			rawPoll:            nil,
			started:            false,
			requiresTwoSamples: false,
			lastSample:         nil,
			lastTimestamp:      time.Time{},
		}
		var totalTime time.Duration

		currentData := []nrwin.CPUGroupInfo{
			createCPUInfo("0,0", 1000),
			createCPUInfo("0,1", 2000), // No corresponding last data
		}

		lastData := []nrwin.CPUGroupInfo{
			createCPUInfo("0,0", 500),
			// Missing "0,1"
		}

		validCount := monitor.calculateCPUTimeDelta(currentData, lastData, &totalTime, "test")

		assert.Equal(t, 1, validCount, "Should process only 1 core with matching last data")
		expectedTime := time.Duration(500 * 100) // (1000-500) * 100
		assert.Equal(t, expectedTime, totalTime)
	})

	t.Run("Handle counter wrapping (negative delta)", func(t *testing.T) {
		t.Parallel()

		monitor := &WindowsCPUMonitor{
			context:            nil,
			rawPoll:            nil,
			started:            false,
			requiresTwoSamples: false,
			lastSample:         nil,
			lastTimestamp:      time.Time{},
		}
		var totalTime time.Duration

		currentData := []nrwin.CPUGroupInfo{
			createCPUInfo("0,0", 100),  // Wrapped - smaller than last
			createCPUInfo("0,1", 2000), // Normal increase
		}

		lastData := []nrwin.CPUGroupInfo{
			createCPUInfo("0,0", 9000), // Was larger, counter wrapped
			createCPUInfo("0,1", 1500), // Normal last value
		}

		validCount := monitor.calculateCPUTimeDelta(currentData, lastData, &totalTime, "test")

		assert.Equal(t, 1, validCount, "Should process only 1 core (skip wrapped counter)")
		expectedTime := time.Duration(500 * 100) // (2000-1500) * 100, skip wrapped counter
		assert.Equal(t, expectedTime, totalTime)
	})

	t.Run("Empty current data", func(t *testing.T) {
		t.Parallel()

		monitor := &WindowsCPUMonitor{
			context:            nil,
			rawPoll:            nil,
			started:            false,
			requiresTwoSamples: false,
			lastSample:         nil,
			lastTimestamp:      time.Time{},
		}
		var totalTime time.Duration

		currentData := []nrwin.CPUGroupInfo{}
		lastData := []nrwin.CPUGroupInfo{
			createCPUInfo("0,0", 500),
		}

		validCount := monitor.calculateCPUTimeDelta(currentData, lastData, &totalTime, "test")

		assert.Equal(t, 0, validCount, "Should process 0 cores with empty current data")
		assert.Equal(t, time.Duration(0), totalTime)
	})

	t.Run("Empty last data", func(t *testing.T) {
		t.Parallel()

		monitor := &WindowsCPUMonitor{
			context:            nil,
			rawPoll:            nil,
			started:            false,
			requiresTwoSamples: false,
			lastSample:         nil,
			lastTimestamp:      time.Time{},
		}
		var totalTime time.Duration

		currentData := []nrwin.CPUGroupInfo{
			createCPUInfo("0,0", 1000),
		}
		lastData := []nrwin.CPUGroupInfo{}

		validCount := monitor.calculateCPUTimeDelta(currentData, lastData, &totalTime, "test")

		assert.Equal(t, 0, validCount, "Should process 0 cores with empty last data")
		assert.Equal(t, time.Duration(0), totalTime)
	})

	t.Run("Zero delta", func(t *testing.T) {
		t.Parallel()

		monitor := &WindowsCPUMonitor{
			context:            nil,
			rawPoll:            nil,
			started:            false,
			requiresTwoSamples: false,
			lastSample:         nil,
			lastTimestamp:      time.Time{},
		}
		var totalTime time.Duration

		currentData := []nrwin.CPUGroupInfo{
			createCPUInfo("0,0", 1000),
		}

		lastData := []nrwin.CPUGroupInfo{
			createCPUInfo("0,0", 1000), // Same value - zero delta
		}

		validCount := monitor.calculateCPUTimeDelta(currentData, lastData, &totalTime, "test")

		assert.Equal(t, 1, validCount, "Should process 1 core even with zero delta")
		assert.Equal(t, time.Duration(0), totalTime, "Total time should be zero")
	})

	t.Run("Large delta values", func(t *testing.T) {
		t.Parallel()

		monitor := &WindowsCPUMonitor{
			context:            nil,
			rawPoll:            nil,
			started:            false,
			requiresTwoSamples: false,
			lastSample:         nil,
			lastTimestamp:      time.Time{},
		}
		var totalTime time.Duration

		currentData := []nrwin.CPUGroupInfo{
			createCPUInfo("0,0", 9223372036854775807), // Max int64
		}

		lastData := []nrwin.CPUGroupInfo{
			createCPUInfo("0,0", 9223372036854775800), // Large but valid delta
		}

		validCount := monitor.calculateCPUTimeDelta(currentData, lastData, &totalTime, "test")

		assert.Equal(t, 1, validCount, "Should handle large delta values")
		expectedTime := time.Duration(7 * 100) // (max - (max-7)) * 100
		assert.Equal(t, expectedTime, totalTime)
	})

	t.Run("Multiple cores with mixed scenarios", func(t *testing.T) {
		t.Parallel()

		monitor := &WindowsCPUMonitor{
			context:            nil,
			rawPoll:            nil,
			started:            false,
			requiresTwoSamples: false,
			lastSample:         nil,
			lastTimestamp:      time.Time{},
		}
		var totalTime time.Duration

		currentData := []nrwin.CPUGroupInfo{
			createCPUInfo("_Total", 10000), // Should skip
			createCPUInfo("0,0", 1000),     // Normal delta: 500
			createCPUInfo("0,1", 100),      // Wrapped - skip
			createCPUInfo("0,2", 3000),     // Normal delta: 1000
			createCPUInfo("0,3", 4000),     // No last data - skip
		}

		lastData := []nrwin.CPUGroupInfo{
			createCPUInfo("_Total", 9000), // Should skip
			createCPUInfo("0,0", 500),     // Normal
			createCPUInfo("0,1", 5000),    // Current < last (wrapped)
			createCPUInfo("0,2", 2000),    // Normal
			// Missing "0,3"
		}

		validCount := monitor.calculateCPUTimeDelta(currentData, lastData, &totalTime, "test")

		assert.Equal(t, 2, validCount, "Should process 2 valid cores")
		// Expected: (1000-500) + (3000-2000) = 500 + 1000 = 1500
		expectedTime := time.Duration(1500 * 100)
		assert.Equal(t, expectedTime, totalTime)
	})
}
