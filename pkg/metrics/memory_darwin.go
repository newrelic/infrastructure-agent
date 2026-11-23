// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metrics

import "github.com/shirou/gopsutil/v3/mem"

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
func swapMemory() (*SwapSample, error) {
	swap, err := mem.SwapMemory()
	if err != nil {
		return nil, err
	}

	return &SwapSample{
		SwapFree:  float64(swap.Free),
		SwapTotal: float64(swap.Total),
		SwapUsed:  float64(swap.Used),
	}, nil
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
		MemoryKernelFree:  floatToReference(float64(memStat.Free)),

		MemoryFreePercent: memoryFreePercent,
		MemoryUsedPercent: memoryUsedPercent,

		SwapSample: *swap,
	}, nil
}
