// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryMonitor_IgnoreReclaimable_Sample(t *testing.T) {
	m := NewMemoryMonitor(true)

	sample, err := m.Sample()
	require.NoError(t, err)

	assert.NotZero(t, sample.MemoryTotal)
	assert.NotZero(t, sample.MemoryFree)
	assert.NotZero(t, sample.MemoryUsed)
	assert.InDelta(t, sample.MemoryTotal, sample.MemoryFree+sample.MemoryUsed, 0.1)

	assert.InDelta(t, sample.SwapTotal, sample.SwapFree+sample.SwapUsed, 0.1)
}

func TestMemoryMonitor_ReclaimableValues(t *testing.T) {
	// Given a Memory Monitor that considers reclaimable as free
	mf := NewMemoryMonitor(true)
	// And a monitor that considers reclaimable as used
	mu := NewMemoryMonitor(false)

	// When they fetch memory samples
	sf, err := mf.Sample()
	require.NoError(t, err)
	su, err := mu.Sample()
	require.NoError(t, err)

	// Then Both report the same total memory
	require.InDelta(t, sf.MemoryTotal, su.MemoryTotal, 0.1)

	// And The monitor that considers reclaimable memory as used should have MemUsed > than the other monitor
	require.True(t, su.MemoryUsed > sf.MemoryUsed,
		"%v (MemoryUsed with reclaimable) should be > %v (MemoryUsed without reclaimable)", su.MemoryUsed, sf.MemoryUsed)

	// And The monitor that considers reclaimable memory as free should have MemFree >= than the other monitor
	require.True(t, sf.MemoryFree > su.MemoryFree,
		"%v (MemoryFree without reclaimable) should be > %v (MemoryFree with reclaimable)", sf.MemoryFree, su.MemoryFree)

}
