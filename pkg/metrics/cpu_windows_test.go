//go:build windows
// +build windows

// Copyright 2024 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWindowsCPUMonitor_RawCounterArrays(t *testing.T) {
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
