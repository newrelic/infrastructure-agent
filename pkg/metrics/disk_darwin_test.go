// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build darwin
// +build darwin

package metrics

import (
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/storage"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDiskFromDevice(t *testing.T) {

	cases := []struct {
		name     string
		device   string
		expected string
		err      error
	}{
		{
			name:     "valid disk",
			device:   "/dev/disk1s1s1",
			expected: "disk1",
		},
		{
			name:     "invalid device",
			device:   "/dev/someweirddevice",
			expected: "",
			err:      fmt.Errorf("cannot match disk from device %s", "/dev/someweirddevice"),
		},
		{
			name:     "APFS System Snapshot",
			device:   "devfs",
			expected: "",
			err:      fmt.Errorf("cannot match disk from device %s", "devfs"),
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			d, err := diskFromDevice(test.device)
			assert.Equal(t, test.err, err)
			assert.Equal(t, test.expected, d)
		})
	}

}

func TestGetUtilizationDataFromIoCountersDelta(t *testing.T) {

	tests := []struct {
		name            string
		elapsedMs       int64
		ioCountersStats map[string]storage.IOCountersStat
		lastDiskStats   map[string]storage.IOCountersStat
		expected        ioCountersData
	}{
		{
			name:      "happy path",
			elapsedMs: 350,
			lastDiskStats: map[string]storage.IOCountersStat{
				"disk1": storage.DarwinIoCountersStat{
					IOCountersStat: disk.IOCountersStat{
						ReadCount:  12,
						WriteCount: 21,
						ReadTime:   34,
						WriteTime:  23,
						IoTime:     34 + 23,
					},
				},
				"disk2": storage.DarwinIoCountersStat{
					IOCountersStat: disk.IOCountersStat{
						ReadCount:  25,
						WriteCount: 45,
						ReadTime:   78,
						WriteTime:  23,
						IoTime:     78 + 23,
					},
				},
			},
			ioCountersStats: map[string]storage.IOCountersStat{
				"disk1": storage.DarwinIoCountersStat{
					IOCountersStat: disk.IOCountersStat{
						ReadCount:  15,
						WriteCount: 27,
						ReadTime:   87,
						WriteTime:  83,
						IoTime:     87 + 83,
					},
				},
				"disk2": storage.DarwinIoCountersStat{
					IOCountersStat: disk.IOCountersStat{
						ReadCount:  35,
						WriteCount: 61,
						ReadTime:   182,
						WriteTime:  89,
						IoTime:     182 + 89,
					},
				},
			},
			expected: ioCountersData{
				readsPerSec:             (15 - 12 + 35 - 25) / (350.0 / 1000.0),
				writesPerSec:            (27 - 21 + 61 - 45) / (350.0 / 1000.0),
				percentUtilized:         (float64(((87+83)-(34+23))+((182+89)-(78+23))) / (350.0 * 2)) * 100,
				readUtilizationPercent:  (float64((87-34)+(182-78)) / float64(((87+83)-(34+23))+((182+89)-(78+23)))) * (float64(((87+83)-(34+23))+((182+89)-(78+23))) / (350.0 * 2)) * 100,
				writeUtilizationPercent: (float64((83-23)+(89-23)) / float64(((87+83)-(34+23))+((182+89)-(78+23)))) * (float64(((87+83)-(34+23))+((182+89)-(78+23))) / (350.0 * 2)) * 100,
			},
		},
		{
			name:      "no delta time",
			elapsedMs: 0,
			lastDiskStats: map[string]storage.IOCountersStat{
				"disk1": storage.DarwinIoCountersStat{
					IOCountersStat: disk.IOCountersStat{
						ReadCount:  12,
						WriteCount: 21,
						ReadTime:   34,
						WriteTime:  23,
						IoTime:     34 + 23,
					},
				},
				"disk2": storage.DarwinIoCountersStat{
					IOCountersStat: disk.IOCountersStat{
						ReadCount:  25,
						WriteCount: 45,
						ReadTime:   78,
						WriteTime:  23,
						IoTime:     78 + 23,
					},
				},
			},
			ioCountersStats: map[string]storage.IOCountersStat{
				"disk1": storage.DarwinIoCountersStat{
					IOCountersStat: disk.IOCountersStat{
						ReadCount:  15,
						WriteCount: 27,
						ReadTime:   87,
						WriteTime:  83,
						IoTime:     87 + 83,
					},
				},
				"disk2": storage.DarwinIoCountersStat{
					IOCountersStat: disk.IOCountersStat{
						ReadCount:  35,
						WriteCount: 61,
						ReadTime:   182,
						WriteTime:  89,
						IoTime:     182 + 89,
					},
				},
			},
			expected: ioCountersData{},
		},
		{
			name:      "no last stats",
			elapsedMs: 0,
			lastDiskStats: map[string]storage.IOCountersStat{
				"disk1": storage.DarwinIoCountersStat{
					IOCountersStat: disk.IOCountersStat{
						ReadCount:  12,
						WriteCount: 21,
						ReadTime:   34,
						WriteTime:  23,
						IoTime:     34 + 23,
					},
				},
				"disk2": storage.DarwinIoCountersStat{
					IOCountersStat: disk.IOCountersStat{
						ReadCount:  25,
						WriteCount: 45,
						ReadTime:   78,
						WriteTime:  23,
						IoTime:     78 + 23,
					},
				},
			},
			ioCountersStats: map[string]storage.IOCountersStat{},
			expected:        ioCountersData{},
		},
		{
			name:      "no stats",
			elapsedMs: 0,
			lastDiskStats: map[string]storage.IOCountersStat{
				"disk1": storage.DarwinIoCountersStat{
					IOCountersStat: disk.IOCountersStat{
						ReadCount:  15,
						WriteCount: 27,
						ReadTime:   87,
						WriteTime:  83,
						IoTime:     87 + 83,
					},
				},
				"disk2": storage.DarwinIoCountersStat{
					IOCountersStat: disk.IOCountersStat{
						ReadCount:  35,
						WriteCount: 61,
						ReadTime:   182,
						WriteTime:  89,
						IoTime:     182 + 89,
					},
				},
			},
			ioCountersStats: map[string]storage.IOCountersStat{},
			expected:        ioCountersData{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			u := getUtilizationDataFromIoCountersDelta(test.elapsedMs, test.ioCountersStats, test.lastDiskStats)
			delta := 0.00001
			assert.InDelta(t, test.expected.readsPerSec, u.readsPerSec, delta)
			assert.InDelta(t, test.expected.writesPerSec, u.writesPerSec, delta)
			assert.InDelta(t, test.expected.percentUtilized, u.percentUtilized, delta)
			assert.InDelta(t, test.expected.readUtilizationPercent, u.readUtilizationPercent, delta)
			assert.InDelta(t, test.expected.writeUtilizationPercent, u.writeUtilizationPercent, delta)
		})
	}

}

func TestGetStorageData(t *testing.T) {
	tests := []struct {
		name     string
		samples  []*storage.Sample
		expected storageData
	}{
		{
			name:     "no samples",
			samples:  []*storage.Sample{},
			expected: storageData{},
		},
		{
			name: "sample with not UsedBytes",
			samples: []*storage.Sample{
				{
					storage.BaseSample{
						Device:         "/dev/disk1s1",
						FileSystemType: "hfs",
						TotalBytes:     fp(10000),
						FreeBytes:      fp(2500),
					},
				},
			},
			expected: storageData{},
		},
		{
			name: "sample with not FreeBytes",
			samples: []*storage.Sample{
				{
					storage.BaseSample{
						Device:         "/dev/disk1s1",
						FileSystemType: "hfs",
						TotalBytes:     fp(10000),
						UsedBytes:      fp(7500),
					},
				},
			},
			expected: storageData{},
		},
		{
			name: "sample with not TotalBytes",
			samples: []*storage.Sample{
				{
					storage.BaseSample{
						Device:         "/dev/disk1s1",
						FileSystemType: "hfs",
						FreeBytes:      fp(2500),
						UsedBytes:      fp(7500),
					},
				},
			},
			expected: storageData{},
		},
		{
			name: "one non apfs partition",
			samples: []*storage.Sample{
				{
					storage.BaseSample{
						Device:         "/dev/disk1s1",
						FileSystemType: "hfs",
						TotalBytes:     fp(10000),
						FreeBytes:      fp(2500),
						UsedBytes:      fp(7500),
					},
				},
			},
			expected: storageData{
				totalUsedBytes:  7500,
				totalFreeBytes:  2500,
				totalBytes:      10000,
				diskUsedPercent: 75,
				diskFreePercent: 25,
			},
		},
		{
			name: "multiple non apfs partition",
			samples: []*storage.Sample{
				{
					storage.BaseSample{
						Device:         "/dev/disk1s1",
						FileSystemType: "hfs",
						TotalBytes:     fp(10000),
						FreeBytes:      fp(2500),
						UsedBytes:      fp(7500),
					},
				},
				{
					storage.BaseSample{
						Device:         "/dev/disk1s3",
						FileSystemType: "hfs",
						TotalBytes:     fp(20000),
						FreeBytes:      fp(200),
						UsedBytes:      fp(19800),
					},
				},
			},
			expected: storageData{
				totalUsedBytes:  7500 + 19800,
				totalFreeBytes:  2500 + 200,
				totalBytes:      30000,
				diskUsedPercent: 91,
				diskFreePercent: 9,
			},
		},
		{
			name: "multiple with apfs partition",
			samples: []*storage.Sample{
				{
					storage.BaseSample{
						Device:         "/dev/disk1s1s1",
						FileSystemType: "apfs",
						TotalBytes:     fp(10000),
						FreeBytes:      fp(2500),
						UsedBytes:      fp(100),
					},
				},
				{
					storage.BaseSample{
						Device:         "/dev/disk1s2",
						FileSystemType: "apfs",
						TotalBytes:     fp(10000),
						FreeBytes:      fp(2500),
						UsedBytes:      fp(900),
					},
				},
				{
					storage.BaseSample{
						Device:         "/dev/disk1s3",
						FileSystemType: "apfs",
						TotalBytes:     fp(10000),
						FreeBytes:      fp(2500),
						UsedBytes:      fp(6500),
					},
				},
			},
			expected: storageData{
				totalUsedBytes:  100 + 900 + 6500,
				totalFreeBytes:  2500,
				totalBytes:      10000,
				diskUsedPercent: 75,
				diskFreePercent: 25,
			},
		},
		{
			name: "multiple partitions types",
			samples: []*storage.Sample{
				{
					storage.BaseSample{
						Device:         "/dev/disk1s1s1",
						FileSystemType: "apfs",
						TotalBytes:     fp(10000),
						FreeBytes:      fp(5000),
						UsedBytes:      fp(100),
					},
				},
				{
					storage.BaseSample{
						Device:         "/dev/disk1s2",
						FileSystemType: "apfs",
						TotalBytes:     fp(10000),
						FreeBytes:      fp(5000),
						UsedBytes:      fp(900),
					},
				},
				{
					storage.BaseSample{
						Device:         "/dev/disk1s3",
						FileSystemType: "apfs",
						TotalBytes:     fp(10000),
						FreeBytes:      fp(5000),
						UsedBytes:      fp(4000),
					},
				},
				{
					storage.BaseSample{
						Device:         "/dev/disk2s1",
						FileSystemType: "hps",
						TotalBytes:     fp(20000),
						FreeBytes:      fp(1000),
						UsedBytes:      fp(19000),
					},
				},
			},
			expected: storageData{
				totalUsedBytes:  100 + 900 + 4000 + 19000,
				totalFreeBytes:  5000 + 1000,
				totalBytes:      10000 + 20000,
				diskUsedPercent: 80,
				diskFreePercent: 20,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, getStorageData(test.samples))
		})
	}
}

func fp(f float64) *float64 {
	return &f
}
