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
