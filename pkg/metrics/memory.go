// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metrics

import (
	"fmt"
	"runtime/debug"

	"github.com/shirou/gopsutil/v3/mem"
)

type SwapSample struct {
	SwapTotal float64 `json:"swapTotalBytes"`
	SwapFree  float64 `json:"swapFreeBytes"`
	SwapUsed  float64 `json:"swapUsedBytes"`
	// only available (gopsutil) in Linux
	SwapIn  *float64 `json:"swapInBytes,omitempty"`
	SwapOut *float64 `json:"swapOutBytes,omitempty"`
}

type MemorySample struct {
	MemoryTotal       float64  `json:"memoryTotalBytes"`
	MemoryFree        float64  `json:"memoryFreeBytes"`
	MemoryUsed        float64  `json:"memoryUsedBytes"`
	MemoryFreePercent float64  `json:"memoryFreePercent"`
	MemoryUsedPercent float64  `json:"memoryUsedPercent"`
	MemoryCachedBytes float64  `json:"memoryCachedBytes"`
	MemorySlabBytes   float64  `json:"memorySlabBytes"`
	MemorySharedBytes float64  `json:"memorySharedBytes"`
	MemoryBuffers     *float64 `json:"memoryBuffers,omitempty"`
	MemoryKernelFree  *float64 `json:"memoryKernelFree,omitempty"`
	SwapSample
}

type MemoryMonitor struct {
	vmHarvest func() (*mem.VirtualMemoryStat, error)
}

func (mm *MemoryMonitor) Sample() (result *MemorySample, err error) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = fmt.Errorf("Panic in MemoryMonitor.Sample: %v\nStack: %s", panicErr, debug.Stack())
		}
	}()

	memory, err := mm.vmHarvest()
	if err != nil {
		return nil, err
	}

	swap, err := swapMemory()
	if err != nil {
		return nil, err
	}

	memoryFreePercent := float64(0)
	memoryUsedPercent := float64(0)
	if memory.Total > 0 {
		memoryFreePercent = float64(memory.Available) / float64(memory.Total) * 100
		memoryUsedPercent = 100.0 - memoryFreePercent
	}

	return memorySample(memory, swap, memoryFreePercent, memoryUsedPercent)
}

func floatToReference(value float64) *float64 {
	return &value
}
