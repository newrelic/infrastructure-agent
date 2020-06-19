// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build windows

package storage

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWmiCalculateBytesRate(t *testing.T) {
	lastStats := &WmiIoCountersStat{
		Raw: Win32_PerfRawData_PerfDisk_LogicalDisk{
			Name:                 "nameeee",
			DiskReadsPerSec:      0,
			DiskReadBytesPerSec:  0,
			DiskWritesPerSec:     0,
			DiskWriteBytesPerSec: 0,
			Frequency_PerfTime:   0,
			Timestamp_PerfTime:   0,
		},
		Formatted: Win32_PerfFormattedData_PerfDisk_LogicalDisk{
			Name:                 "nameeee",
			PercentDiskTime:      0,
			PercentDiskReadTime:  0,
			PercentDiskWriteTime: 0,
		},
	}
	counter := &WmiIoCountersStat{
		Raw: Win32_PerfRawData_PerfDisk_LogicalDisk{
			Name:                 "nameeee",
			DiskReadsPerSec:      1000,
			DiskReadBytesPerSec:  1000,
			DiskWritesPerSec:     1000,
			DiskWriteBytesPerSec: 1000,
			Frequency_PerfTime:   1000,
			Timestamp_PerfTime:   1000,
		},
		Formatted: Win32_PerfFormattedData_PerfDisk_LogicalDisk{
			Name:                 "nameeee",
			PercentDiskTime:      100,
			PercentDiskReadTime:  90,
			PercentDiskWriteTime: 80,
		},
	}
	elapsedMS := int64(1500)
	ioSample := CalculateWmiSampleValues(counter, lastStats, elapsedMS)

	assert.InEpsilon(t, 1000, *ioSample.ReadBytesPerSec, 0.01)
	assert.InEpsilon(t, 1000, *ioSample.WriteBytesPerSec, 0.01)
	assert.InEpsilon(t, 1000, *ioSample.ReadsPerSec, 0.01)
	assert.InEpsilon(t, 1000, *ioSample.WritesPerSec, 0.01)

	assert.InEpsilon(t, 100, *ioSample.TotalUtilizationPercent, 0.01)
	assert.InEpsilon(t, 90, *ioSample.ReadUtilizationPercent, 0.01)
	assert.InEpsilon(t, 80, *ioSample.WriteUtilizationPercent, 0.01)
}

func TestWmiMarshallableSamples(t *testing.T) {
	testCases := []struct {
		elapsedTime int64
		description string
		counter     *WmiIoCountersStat
		lastStats   *WmiIoCountersStat
	}{
		{
			1000, "normal operation",
			&WmiIoCountersStat{
				Win32_PerfRawData_PerfDisk_LogicalDisk{
					Name: "nameeee", DiskReadsPerSec: 1000, DiskReadBytesPerSec: 1000, DiskWritesPerSec: 1000,
					DiskWriteBytesPerSec: 1000, Frequency_PerfTime: 1000, Timestamp_PerfTime: 1000,
				},
				Win32_PerfFormattedData_PerfDisk_LogicalDisk{
					Name: "nameeee", PercentDiskTime: 100, PercentDiskReadTime: 90, PercentDiskWriteTime: 80,
				},
			},
			&WmiIoCountersStat{
				Win32_PerfRawData_PerfDisk_LogicalDisk{
					Name: "nameeee", DiskReadsPerSec: 0, DiskReadBytesPerSec: 0, DiskWritesPerSec: 0,
					DiskWriteBytesPerSec: 0, Frequency_PerfTime: 0, Timestamp_PerfTime: 0,
				},
				Win32_PerfFormattedData_PerfDisk_LogicalDisk{
					Name: "nameeee", PercentDiskTime: 0, PercentDiskReadTime: 0, PercentDiskWriteTime: 0,
				},
			},
		},
		{
			0, "with 0 elapsed time",
			&WmiIoCountersStat{
				Win32_PerfRawData_PerfDisk_LogicalDisk{
					Name: "nameeee", DiskReadsPerSec: 1000, DiskReadBytesPerSec: 1000, DiskWritesPerSec: 1000,
					DiskWriteBytesPerSec: 1000, Frequency_PerfTime: 1000, Timestamp_PerfTime: 1000,
				},
				Win32_PerfFormattedData_PerfDisk_LogicalDisk{
					Name: "nameeee", PercentDiskTime: 100, PercentDiskReadTime: 90, PercentDiskWriteTime: 80,
				},
			}, &WmiIoCountersStat{
				Win32_PerfRawData_PerfDisk_LogicalDisk{
					Name: "nameeee", DiskReadsPerSec: 0, DiskReadBytesPerSec: 0, DiskWritesPerSec: 0,
					DiskWriteBytesPerSec: 0, Frequency_PerfTime: 0, Timestamp_PerfTime: 0,
				},
				Win32_PerfFormattedData_PerfDisk_LogicalDisk{
					Name: "nameeee", PercentDiskTime: 0, PercentDiskReadTime: 0, PercentDiskWriteTime: 0,
				},
			},
		},
		{
			1000, "with 0 perf times",
			&WmiIoCountersStat{
				Win32_PerfRawData_PerfDisk_LogicalDisk{
					Name: "nameeee", DiskReadsPerSec: 1000, DiskReadBytesPerSec: 1000, DiskWritesPerSec: 1000,
					DiskWriteBytesPerSec: 1000, Frequency_PerfTime: 0, Timestamp_PerfTime: 0,
				},
				Win32_PerfFormattedData_PerfDisk_LogicalDisk{
					Name: "nameeee", PercentDiskTime: 100, PercentDiskReadTime: 90, PercentDiskWriteTime: 80,
				},
			},
			&WmiIoCountersStat{
				Win32_PerfRawData_PerfDisk_LogicalDisk{
					Name: "nameeee", DiskReadsPerSec: 0, DiskReadBytesPerSec: 0, DiskWritesPerSec: 0,
					DiskWriteBytesPerSec: 0, Frequency_PerfTime: 0, Timestamp_PerfTime: 0,
				},
				Win32_PerfFormattedData_PerfDisk_LogicalDisk{
					Name: "nameeee", PercentDiskTime: 0, PercentDiskReadTime: 0, PercentDiskWriteTime: 0,
				},
			},
		},
		{
			1000, "with 0 frequency perf times",
			&WmiIoCountersStat{
				Win32_PerfRawData_PerfDisk_LogicalDisk{
					Name: "nameeee", DiskReadsPerSec: 1000, DiskReadBytesPerSec: 1000, DiskWritesPerSec: 1000,
					DiskWriteBytesPerSec: 1000, Frequency_PerfTime: 0, Timestamp_PerfTime: 1000,
				},
				Win32_PerfFormattedData_PerfDisk_LogicalDisk{
					Name: "nameeee", PercentDiskTime: 100, PercentDiskReadTime: 90, PercentDiskWriteTime: 80,
				},
			},
			&WmiIoCountersStat{
				Win32_PerfRawData_PerfDisk_LogicalDisk{
					Name: "nameeee", DiskReadsPerSec: 0, DiskReadBytesPerSec: 0, DiskWritesPerSec: 0,
					DiskWriteBytesPerSec: 0, Frequency_PerfTime: 0, Timestamp_PerfTime: 0,
				},
				Win32_PerfFormattedData_PerfDisk_LogicalDisk{
					Name: "nameeee", PercentDiskTime: 0, PercentDiskReadTime: 0, PercentDiskWriteTime: 0,
				},
			},
		},
		{
			1000, "with 0 or nil in all values",
			&WmiIoCountersStat{
				Win32_PerfRawData_PerfDisk_LogicalDisk{Name: "nameeee"},
				Win32_PerfFormattedData_PerfDisk_LogicalDisk{Name: "nameeee"},
			},
			&WmiIoCountersStat{
				Win32_PerfRawData_PerfDisk_LogicalDisk{Name: "nameeee"},
				Win32_PerfFormattedData_PerfDisk_LogicalDisk{Name: "nameeee"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			ioSample := CalculateWmiSampleValues(tc.counter, tc.lastStats, tc.elapsedTime)
			_, err := json.Marshal(ioSample)
			assert.NoError(t, err)
		})
	}
}

func TestWmiMarshallableSamplesFromJSON(t *testing.T) {
	testCases := []struct {
		counter   string
		lastStats string
	}{
		{
			"{\"Raw\":{\"Name\":\"F:\",\"DiskReadsPerSec\":41,\"DiskReadBytesPerSec\":236032,\"DiskWritesPerSec\":1137," +
				"\"DiskWriteBytesPerSec\":465825792,\"Frequency_PerfTime\":10000000,\"Timestamp_PerfTime\":718194782285}" +
				",\"Formatted\":{\"Name\":\"F:\",\"PercentDiskTime\":0,\"PercentDiskReadTime\":0,\"PercentDiskWriteTime\":0}}",
			"{\"Raw\":{\"Name\":\"F:\",\"DiskReadsPerSec\":41,\"DiskReadBytesPerSec\":236032,\"DiskWritesPerSec\":1137," +
				"\"DiskWriteBytesPerSec\":465825792,\"Frequency_PerfTime\":10000000,\"Timestamp_PerfTime\":718185020579}," +
				"\"Formatted\":{\"Name\":\"F:\",\"PercentDiskTime\":0,\"PercentDiskReadTime\":0,\"PercentDiskWriteTime\":0}}",
		},
		{
			"{\"Raw\":{\"Name\":\"G:\",\"DiskReadsPerSec\":595381,\"DiskReadBytesPerSec\":36094513152,\"DiskWritesPerSec\":47923," +
				"\"DiskWriteBytesPerSec\":2213957632,\"Frequency_PerfTime\":10000000,\"Timestamp_PerfTime\":718194782285}," +
				"\"Formatted\":{\"Name\":\"G:\",\"PercentDiskTime\":0,\"PercentDiskReadTime\":0,\"PercentDiskWriteTime\":0}}",
			"{\"Raw\":{\"Name\":\"G:\",\"DiskReadsPerSec\":595381,\"DiskReadBytesPerSec\":36094513152,\"DiskWritesPerSec\":47923," +
				"\"DiskWriteBytesPerSec\":2213957632,\"Frequency_PerfTime\":10000000,\"Timestamp_PerfTime\":718185020579}," +
				"\"Formatted\":{\"Name\":\"G:\",\"PercentDiskTime\":0,\"PercentDiskReadTime\":0,\"PercentDiskWriteTime\":0}}",
		},
		{
			"{\"Raw\":{\"Name\":\"H:\",\"DiskReadsPerSec\":5188,\"DiskReadBytesPerSec\":3047456768,\"DiskWritesPerSec\":93111," +
				"\"DiskWriteBytesPerSec\":3236696064,\"Frequency_PerfTime\":10000000,\"Timestamp_PerfTime\":718194782285}," +
				"\"Formatted\":{\"Name\":\"H:\",\"PercentDiskTime\":0,\"PercentDiskReadTime\":0,\"PercentDiskWriteTime\":0}}",
			"{\"Raw\":{\"Name\":\"H:\",\"DiskReadsPerSec\":5188,\"DiskReadBytesPerSec\":3047456768,\"DiskWritesPerSec\":93110," +
				"\"DiskWriteBytesPerSec\":3236691968,\"Frequency_PerfTime\":10000000,\"Timestamp_PerfTime\":718185020579}," +
				"\"Formatted\":{\"Name\":\"H:\",\"PercentDiskTime\":0,\"PercentDiskReadTime\":0,\"PercentDiskWriteTime\":0}}",
		},
		{
			"{\"Raw\":{\"Name\":\"C:\",\"DiskReadsPerSec\":365539,\"DiskReadBytesPerSec\":15816530432,\"DiskWritesPerSec\":627559," +
				"\"DiskWriteBytesPerSec\":16758744576,\"Frequency_PerfTime\":10000000,\"Timestamp_PerfTime\":718194782285}," +
				"\"Formatted\":{\"Name\":\"C:\",\"PercentDiskTime\":0,\"PercentDiskReadTime\":0,\"PercentDiskWriteTime\":0}}",
			"{\"Raw\":{\"Name\":\"C:\",\"DiskReadsPerSec\":365521,\"DiskReadBytesPerSec\":15815350784,\"DiskWritesPerSec\":627551," +
				"\"DiskWriteBytesPerSec\":16758334976,\"Frequency_PerfTime\":10000000,\"Timestamp_PerfTime\":718185020579}," +
				"\"Formatted\":{\"Name\":\"C:\",\"PercentDiskTime\":0,\"PercentDiskReadTime\":0,\"PercentDiskWriteTime\":0}}",
		},
		{
			"{\"Raw\":{\"Name\":\"D:\",\"DiskReadsPerSec\":4803,\"DiskReadBytesPerSec\":148185088,\"DiskWritesPerSec\":9769," +
				"\"DiskWriteBytesPerSec\":1707168256,\"Frequency_PerfTime\":10000000,\"Timestamp_PerfTime\":718194782285}," +
				"\"Formatted\":{\"Name\":\"D:\",\"PercentDiskTime\":0,\"PercentDiskReadTime\":0,\"PercentDiskWriteTime\":0}}",
			"{\"Raw\":{\"Name\":\"D:\",\"DiskReadsPerSec\":4803,\"DiskReadBytesPerSec\":148185088,\"DiskWritesPerSec\":9769," +
				"\"DiskWriteBytesPerSec\":1707168256,\"Frequency_PerfTime\":10000000,\"Timestamp_PerfTime\":718185020579}," +
				"\"Formatted\":{\"Name\":\"D:\",\"PercentDiskTime\":0,\"PercentDiskReadTime\":0,\"PercentDiskWriteTime\":0}}",
		},
	}
	for i, tc := range testCases {
		t.Run(fmt.Sprintf("JSON %v", i), func(t *testing.T) {
			counter := WmiIoCountersStat{}
			assert.NoError(t, json.Unmarshal([]byte(tc.counter), &counter))
			lastStats := WmiIoCountersStat{}
			assert.NoError(t, json.Unmarshal([]byte(tc.lastStats), &lastStats))
			ioSample := CalculateWmiSampleValues(&counter, &lastStats, 1000)
			_, err := json.Marshal(ioSample)
			assert.NoError(t, err)
		})
	}

}

func TestWmiDiskIOCounters(t *testing.T) {
	ret, err := WmiIoCounters()
	assert.NoError(t, err)
	assert.NotEqual(t, len(ret), 0)

	empty := WmiIoCountersStat{}
	for _, io := range ret {
		pio := io.(*WmiIoCountersStat)
		assert.NotEqual(t, *pio, empty)
	}
}
