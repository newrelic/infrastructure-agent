// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//+build windows

package storage

import (
	"encoding/json"

	"github.com/StackExchange/wmi"
	"github.com/newrelic/infrastructure-agent/pkg/config"
)

// WmiIoCountersStat provides IOCountersStat implementation for WMI
type WmiIoCountersStat struct {
	Raw       Win32_PerfRawData_PerfDisk_LogicalDisk
	Formatted Win32_PerfFormattedData_PerfDisk_LogicalDisk
}

type Win32_PerfRawData_PerfDisk_LogicalDisk struct {
	Name                 string
	DiskReadsPerSec      uint64
	DiskReadBytesPerSec  uint64
	DiskWritesPerSec     uint64
	DiskWriteBytesPerSec uint64
	Frequency_PerfTime   uint64
	Timestamp_PerfTime   uint64
}

type Win32_PerfFormattedData_PerfDisk_LogicalDisk struct {
	Name                 string
	PercentDiskTime      uint64
	PercentDiskReadTime  uint64
	PercentDiskWriteTime uint64
}

func (d *WmiIoCountersStat) String() string {
	s, _ := json.Marshal(*d)
	return string(s)
}

func (d *WmiIoCountersStat) Source() string {
	return "wmi"
}

func CalculateWmiSampleValues(counter *WmiIoCountersStat, lastStats *WmiIoCountersStat, elapsedMs int64) (ioSample *Sample) {
	elapsedSeconds := float64(elapsedMs) / 1000
	result := &Sample{}

	// WMI RAW entities must be "cooked" to display meaningful data. More info on:
	// https://msdn.microsoft.com/en-us/windows/hardware/aa394307(v=vs.71)
	// https://msdn.microsoft.com/en-us/library/windows/desktop/aa371891(v=vs.85).aspx

	perfTimeDiff := counter.Raw.Timestamp_PerfTime - lastStats.Raw.Timestamp_PerfTime
	if perfTimeDiff > 0 {
		readBytes := float64((counter.Raw.DiskReadBytesPerSec-lastStats.Raw.DiskReadBytesPerSec)*counter.Raw.Frequency_PerfTime) /
			float64(perfTimeDiff)
		result.ReadBytesPerSec = &readBytes

		writeBytes := float64((counter.Raw.DiskWriteBytesPerSec-lastStats.Raw.DiskWriteBytesPerSec)*counter.Raw.Frequency_PerfTime) /
			float64(perfTimeDiff)
		result.WriteBytesPerSec = &writeBytes

		if counter.Raw.Frequency_PerfTime > 0 {
			readsPerSec := float64(counter.Raw.DiskReadsPerSec-lastStats.Raw.DiskReadsPerSec) / (float64(perfTimeDiff) / float64(counter.Raw.Frequency_PerfTime))

			result.ReadsPerSec = &readsPerSec

			writesPerSec := float64(counter.Raw.DiskWritesPerSec-lastStats.Raw.DiskWritesPerSec) / (float64(perfTimeDiff) / float64(counter.Raw.Frequency_PerfTime))
			result.WritesPerSec = &writesPerSec

			result.HasDelta = true
			result.ReadCountDelta = uint64(readsPerSec * elapsedSeconds)
			result.WriteCountDelta = uint64(writesPerSec * elapsedSeconds)
		}

	}

	var total, read, write float64

	total = float64(counter.Formatted.PercentDiskTime)
	read = float64(counter.Formatted.PercentDiskReadTime)
	write = float64(counter.Formatted.PercentDiskWriteTime)

	result.TotalUtilizationPercent = &total
	result.ReadUtilizationPercent = &read
	result.WriteUtilizationPercent = &write

	return result
}

func WmiIoCounters() (map[string]IOCountersStat, error) {
	ret := make(map[string]IOCountersStat, 0)

	var raw []Win32_PerfRawData_PerfDisk_LogicalDisk
	q := wmi.CreateQuery(&raw, "")
	err := wmi.QueryNamespace(q, &raw, config.DefaultWMINamespace)
	if err != nil {
		return ret, err
	}
	for _, d := range raw {
		if len(d.Name) > 3 { // not get _Total or Harddrive
			continue
		}
		ret[d.Name] = &WmiIoCountersStat{Raw: d}
	}

	var formatted []Win32_PerfFormattedData_PerfDisk_LogicalDisk
	q = wmi.CreateQuery(&formatted, "")
	err = wmi.QueryNamespace(q, &formatted, config.DefaultWMINamespace)
	if err != nil {
		return ret, err
	}
	for _, d := range formatted {
		if len(d.Name) > 3 { // not get _Total or Harddrive
			continue
		}

		r, ok := ret[d.Name]
		if ok {
			ret[d.Name] = &WmiIoCountersStat{Formatted: d, Raw: r.(*WmiIoCountersStat).Raw}
		} else {
			ret[d.Name] = &WmiIoCountersStat{Formatted: d}
		}
	}

	return ret, nil
}
