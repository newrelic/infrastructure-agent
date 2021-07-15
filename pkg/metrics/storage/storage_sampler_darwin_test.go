// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build darwin

package storage

import (
	"github.com/shirou/gopsutil/disk"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDarwinIoCountersStat_Source(t *testing.T) {
	st := DarwinIoCountersStat{}
	assert.Equal(t, "gopsutil", st.Source())
}

func Test_isSupportedDevice(t *testing.T) {
	tests := []struct {
		device    string
		supported bool
	}{
		{"/dev/disk0s1", true},
		{"/dev/disk1s1s2", true},
		{"/dev/disk1", false},
		{"/dev/disk1s2nnn", false},
		{"/devfs", false},
		{"/drivefs", false},
	}

	for _, test := range tests {
		t.Run(test.device, func(t *testing.T) {
			assert.Equal(t, test.supported, isSupportedDevice(test.device))
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
					Opts:       "ro,multilabel,nocluster",
				},
				{
					Device:     "drivefs",
					Mountpoint: "/Volumes/GoogleDrive",
					Fstype:     "dfsfuse_DFS",
					Opts:       "ro,multilabel,nocluster",
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
					Opts:       "ro,multilabel,nocluster",
				},
				{
					Device:     "drivefs",
					Mountpoint: "/Volumes/GoogleDrive",
					Fstype:     "dfsfuse_DFS",
					Opts:       "ro,multilabel,nocluster",
				},
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
