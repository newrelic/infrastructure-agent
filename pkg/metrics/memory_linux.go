// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metrics

import (
	"strconv"
	"strings"

	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/acquire"
	"github.com/shirou/gopsutil/v3/mem"
)

// NewMemoryMonitor returns a memory monitor.
// If consistentMemory is true, the reported free memory is reported as:
// total - free - buffers - cached - sreclaimable, as a consistent implementation that does not change between
// different kernel versions or library implementations.
// If consistentMemory is false, it reports the free memory as the Available Memory, dependant on the current kernel
// or library implementations.
func NewMemoryMonitor(ignoreReclaimable bool) *MemoryMonitor {
	mm := &MemoryMonitor{}
	if ignoreReclaimable {
		mm.vmHarvest = reclaimableAsFree
	} else {
		mm.vmHarvest = reclaimableAsUsed
	}
	return mm
}

// Returns a formulation of the virtual memory that considers SReclaimable as Available, concretely:
// Total Memory: MemTotal
// Available Memory: MemFree + Buffers + Cached + SReclaimable
// Used Memory: Total Memory - Available Memory
func reclaimableAsFree() (*mem.VirtualMemoryStat, error) {
	filename := helpers.HostProc("meminfo")
	lines, _ := acquire.ReadLines(filename)

	return reclaimableAsFreeParseMemInfo(lines)
}

func reclaimableAsFreeParseMemInfo(lines []string) (*mem.VirtualMemoryStat, error) {
	ret := &mem.VirtualMemoryStat{}
	readFields := 0
	for _, line := range lines {
		fields := strings.Split(line, ":")
		if len(fields) != 2 {
			continue
		}
		key := strings.TrimSpace(fields[0])
		value := strings.TrimSpace(fields[1])
		value = strings.Replace(value, " kB", "", -1)

		t, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return ret, err
		}
		switch key {
		case "MemTotal":
			ret.Total = t * 1024
			readFields++
		case "MemFree":
			ret.Free = t * 1024
			readFields++
		case "Buffers":
			ret.Buffers = t * 1024
			readFields++
		case "Cached":
			ret.Cached = t * 1024
			readFields++
		case "Shmem":
			ret.Shared = t * 1024
			readFields++
		case "Slab":
			ret.Slab = t * 1024
			readFields++
		case "SReclaimable":
			ret.Sreclaimable = t * 1024
			readFields++
		}
		if readFields >= 7 { // stop reading the file when we have read all the fields we require
			break
		}
	}

	ret.Cached += ret.Sreclaimable
	ret.Available = ret.Free + ret.Buffers + ret.Cached
	ret.Used = ret.Total - ret.Available

	return ret, nil
}

// Returns a formulation of the virtual memory that considers SReclaimable as Available, concretely:
// Total Memory: MemTotal
// Available Memory (kernels >= 3.14): MemAvailable
// Available Memory (kernels < 3.14): MemFree + Buffers + Cached
// Used Memory: Total Memory - Available Memory
func reclaimableAsUsed() (*mem.VirtualMemoryStat, error) {
	filename := helpers.HostProc("meminfo")
	lines, _ := acquire.ReadLines(filename)
	return reclaimableAsUsedParseMemInfo(lines)
}

func reclaimableAsUsedParseMemInfo(lines []string) (*mem.VirtualMemoryStat, error) {
	memAvailable := false

	ret := &mem.VirtualMemoryStat{}
	readFields := 0
	for _, line := range lines {
		fields := strings.Split(line, ":")
		if len(fields) != 2 {
			continue
		}
		key := strings.TrimSpace(fields[0])
		value := strings.TrimSpace(fields[1])
		value = strings.Replace(value, " kB", "", -1)

		t, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return ret, err
		}
		switch key {
		case "MemAvailable":
			ret.Available = t * 1024
			memAvailable = true
		case "MemTotal":
			ret.Total = t * 1024
			readFields++
		case "MemFree":
			ret.Free = t * 1024
			readFields++
		case "Buffers":
			ret.Buffers = t * 1024
			readFields++
		case "Cached":
			ret.Cached = t * 1024
			readFields++
		case "Shmem":
			ret.Shared = t * 1024
			readFields++
		case "Slab":
			ret.Slab = t * 1024
			readFields++
		case "SReclaimable":
			ret.Sreclaimable = t * 1024
			readFields++
		}
		if readFields >= 7 && memAvailable { // stop reading the file when we have read all the fields we require
			break
		}
	}
	if !memAvailable {
		ret.Available = ret.Free + ret.Buffers + ret.Cached
	}
	ret.Used = ret.Total - ret.Available

	return ret, nil
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
		SwapIn:    floatToReference(float64(swap.Sin)),
		SwapOut:   floatToReference(float64(swap.Sout)),
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
		MemoryBuffers:     floatToReference(float64(memStat.Buffers)),
		MemoryKernelFree:  floatToReference(float64(memStat.Free)),

		MemoryFreePercent: memoryFreePercent,
		MemoryUsedPercent: memoryUsedPercent,

		SwapSample: *swap,
	}, nil
}
