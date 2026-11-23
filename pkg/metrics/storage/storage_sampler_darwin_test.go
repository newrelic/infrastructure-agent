// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build darwin
// +build darwin

package storage

import (
	"github.com/shirou/gopsutil/v3/disk"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDarwinIoCountersStat_Source(t *testing.T) {
	st := DarwinIoCountersStat{}
	assert.Equal(t, "gopsutil", st.Source())
}

func Test_isSupportedFs(t *testing.T) {
	originalSupportedFilesystems := SupportedFileSystems
	defer func() {
		SupportedFileSystems = originalSupportedFilesystems
	}()
	SupportedFileSystems = map[string]bool{
		"one":   true,
		"three": true,
	}

	tests := []struct {
		fs        string
		supported bool
	}{
		{"one", true},
		{"two", false},
		{"three", true},
	}

	for _, test := range tests {
		t.Run(test.fs, func(t *testing.T) {
			assert.Equal(t, test.supported, isSupportedFs(test.fs))
		})
	}
}

func Test_partitionsFromGopsutilPartitions(t *testing.T) {
	tests := []struct {
		name                   string
		gopsutilPartitionStats []disk.PartitionStat
		partitionStats         []PartitionStat
	}{
		{
			name:                   "empty",
			gopsutilPartitionStats: nil,
			partitionStats:         nil,
		},
		{
			name: "no supported partitions",
			gopsutilPartitionStats: []disk.PartitionStat{
				{
					Device:     "devfs",
					Mountpoint: "/dev",
					Fstype:     "devfs",
					Opts:       []string{"ro", "multilabel", "nocluster"},
				},
				{
					Device:     "drivefs",
					Mountpoint: "/Volumes/GoogleDrive",
					Fstype:     "dfsfuse_DFS",
					Opts:       []string{"ro", "multilabel", "nocluster"},
				},
			},
			partitionStats: nil,
		},
		{
			name: "some supported partitions",
			gopsutilPartitionStats: []disk.PartitionStat{
				{
					Device:     "devfs",
					Mountpoint: "/dev",
					Fstype:     "devfs",
					Opts:       []string{"ro", "multilabel", "nocluster"},
				},
				{
					Device:     "drivefs",
					Mountpoint: "/Volumes/GoogleDrive",
					Fstype:     "dfsfuse_DFS",
					Opts:       []string{"ro", "multilabel", "nocluster"},
				},
				{
					Device:     "/dev/disk1s1s1",
					Mountpoint: "/",
					Fstype:     "apfs",
					Opts:       []string{"ro"},
				},
				{
					Device:     "/dev/disk2s4",
					Mountpoint: "/Volumes/Kingston",
					Fstype:     "exfat",
					Opts:       []string{"ro", "multilabel"},
				},
				{
					Device:     "/dev/disk2s3",
					Mountpoint: "/Volumes/Other",
					Fstype:     "hfs",
					Opts:       []string{"ro", "multilabel", "nocluster"},
				},
			},
			partitionStats: []PartitionStat{
				{
					Device:     "/dev/disk1s1s1",
					Mountpoint: "/",
					Fstype:     "apfs",
					Opts:       "ro",
				},
				{
					Device:     "/dev/disk2s4",
					Mountpoint: "/Volumes/Kingston",
					Fstype:     "exfat",
					Opts:       "ro,multilabel",
				},
				{
					Device:     "/dev/disk2s3",
					Mountpoint: "/Volumes/Other",
					Fstype:     "hfs",
					Opts:       "ro,multilabel,nocluster",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.partitionStats, partitionsFromGopsutilPartitions(test.gopsutilPartitionStats))
		})
	}
}
