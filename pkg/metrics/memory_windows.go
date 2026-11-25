// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metrics

import (
	"fmt"

	"github.com/shirou/gopsutil/v4/mem"
)

// NewMemoryMonitor returns a reference to a memory monitor that reads the memory metrics as reported by the system
func NewMemoryMonitor(_ bool) *MemoryMonitor {
	return &MemoryMonitor{vmHarvest: gopsMemorySample}
}

// returns the virtual memory as reported by the Gopsutil library
func gopsMemorySample() (*mem.VirtualMemoryStat, error) {
	memory, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}
	// we override memory.Used because otherwise memory.Used + memory.Available != memory.Total
	memory.Used = memory.Total - memory.Available
	return memory, nil
}

// returns the available swap metrics.
//
// On Windows swap is reported as commit limit/charge (RAM + pagefile), as the agent did with gopsutil v3 and
// as other agents (e.g. Datadog system.swap.*) do. gopsutil v4 mem.SwapMemory reports the pagefile only, which
// would silently change swapTotalBytes/swapUsedBytes/swapFreeBytes for existing Windows hosts.
func swapMemory() (*SwapSample, error) {
	ex, err := mem.NewExWindows().VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("reading commit limit/charge: %w", err)
	}

	return commitSwapSample(ex.CommitTotal, ex.CommitLimit), nil
}

// commitSwapSample builds the swap sample from the commit charge and limit, both in bytes.
func commitSwapSample(commitTotal, commitLimit uint64) *SwapSample {
	var free uint64
	if commitLimit > commitTotal {
		free = commitLimit - commitTotal
	}

	return &SwapSample{
		SwapFree:  float64(free),
		SwapTotal: float64(commitLimit),
		SwapUsed:  float64(commitTotal),
	}
}

// returns the memory metrics.
func memorySample(memStat *mem.VirtualMemoryStat, swap *SwapSample, memoryFreePercent float64, memoryUsedPercent float64) (*MemorySample, error) {
	return &MemorySample{
		MemoryTotal:       float64(memStat.Total),
		MemoryFree:        float64(memStat.Available),
		MemoryUsed:        float64(memStat.Used),
		MemoryCachedBytes: float64(memStat.Cached),
		MemorySlabBytes:   float64(memStat.Slab),
		MemorySharedBytes: float64(memStat.Shared),

		MemoryFreePercent: memoryFreePercent,
		MemoryUsedPercent: memoryUsedPercent,

		SwapSample: *swap,
	}, nil
}
