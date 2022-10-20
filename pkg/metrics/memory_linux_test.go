// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metrics

import (
	"strings"
	"testing"

	"github.com/shirou/gopsutil/v3/mem"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryMonitor_SampleLInux(t *testing.T) {
	t.Parallel()
	m := NewMemoryMonitor(false)

	sample, err := m.Sample()
	require.NoError(t, err)

	// linux specific values
	assert.NotZero(t, sample.MemoryKernelFree)
	assert.NotZero(t, sample.MemoryBuffers)
}

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

func TestNotNullSwapMemory(t *testing.T) {
	m := NewMemoryMonitor(true)

	sample, err := m.Sample()
	require.NoError(t, err)

	assert.NotNil(t, sample.SwapIn)
	assert.NotNil(t, sample.SwapOut)

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

var memInfoWithMemAvailable = `MemTotal:        2040788 kB
MemFree:         1344288 kB
MemAvailable:    1595120 kB
Buffers:           59388 kB
Cached:           292896 kB
SwapCached:            0 kB
Active:           388176 kB
Inactive:         177040 kB
Active(anon):     203872 kB
Inactive(anon):      144 kB
Active(file):     184304 kB
Inactive(file):   176896 kB
Unevictable:       18476 kB
Mlocked:           18476 kB
SwapTotal:             0 kB
SwapFree:              0 kB
Dirty:               248 kB
Writeback:           184 kB
AnonPages:        231428 kB
Mapped:           163992 kB
Shmem:              1044 kB
Slab:              79668 kB
SReclaimable:      42636 kB
SUnreclaim:        37032 kB
KernelStack:        3984 kB
PageTables:         7012 kB
NFS_Unstable:          0 kB
Bounce:                0 kB
WritebackTmp:          0 kB
CommitLimit:     1020392 kB
Committed_AS:    1794120 kB
VmallocTotal:   34359738367 kB
VmallocUsed:           0 kB
VmallocChunk:          0 kB
HardwareCorrupted:     0 kB
AnonHugePages:         0 kB
ShmemHugePages:        0 kB
ShmemPmdMapped:        0 kB
CmaTotal:              0 kB
CmaFree:               0 kB
HugePages_Total:       0
HugePages_Free:        0
HugePages_Rsvd:        0
HugePages_Surp:        0
Hugepagesize:       2048 kB
DirectMap4k:      122816 kB
DirectMap2M:     1974272 kB
`

func TestMemoryMonitor_reclaimableAsUsedParseMemInfo(t *testing.T) {
	actual, err := reclaimableAsUsedParseMemInfo(strings.Split(memInfoWithMemAvailable, "\n"))
	assert.NoError(t, err)
	expected := &mem.VirtualMemoryStat{
		Available:    1595120 * 1024,
		Total:        2040788 * 1024,
		Free:         1344288 * 1024,
		Buffers:      59388 * 1024,
		Cached:       292896 * 1024,
		Shared:       1044 * 1024,
		Slab:         79668 * 1024,
		Sreclaimable: 42636 * 1024,
		Used:         (2040788 - 1595120) * 1024, // Total - Available
	}
	assert.Equal(t, expected.String(), actual.String())
}

func TestMemoryMonitor_reclaimableAsUsedParseMemInfoWithoutMemAvailable(t *testing.T) {
	lines := strings.Split(memInfoWithMemAvailable, "\n")
	// Remove MemAvailable line
	lines = append(lines[:2], lines[3:]...)

	memAvailable := uint64(1344288 + 59388 + 292896) // Free + Buffers + Cached

	actual, err := reclaimableAsUsedParseMemInfo(lines)
	assert.NoError(t, err)
	expected := &mem.VirtualMemoryStat{
		Available:    memAvailable * 1024,
		Total:        2040788 * 1024,
		Free:         1344288 * 1024,
		Buffers:      59388 * 1024,
		Cached:       292896 * 1024,
		Shared:       1044 * 1024,
		Slab:         79668 * 1024,
		Sreclaimable: 42636 * 1024,
		Used:         (2040788 - memAvailable) * 1024, // Total - Available
	}
	assert.Equal(t, expected.String(), actual.String())
}

func TestMemoryMonitor_reclaimableAsFreeParseMemInfo(t *testing.T) {
	actual, err := reclaimableAsFreeParseMemInfo(strings.Split(memInfoWithMemAvailable, "\n"))
	assert.NoError(t, err)

	memAvailable := uint64(1344288 + 59388 + 292896 + 42636) // Free + Buffers + Cached + SReclaimable
	expected := &mem.VirtualMemoryStat{
		Available:    memAvailable * 1024,
		Total:        2040788 * 1024,
		Free:         1344288 * 1024,
		Buffers:      59388 * 1024,
		Cached:       (292896 + 42636) * 1024, // Cached + SReclaimable
		Shared:       1044 * 1024,
		Slab:         79668 * 1024,
		Sreclaimable: 42636 * 1024,
		Used:         (2040788 - memAvailable) * 1024, // Total - Available
	}
	assert.Equal(t, expected.String(), actual.String())
}
