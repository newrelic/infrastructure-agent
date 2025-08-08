// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux
// +build linux

package storage

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/newrelic/infrastructure-agent/internal/agent/mocks"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/shirou/gopsutil/v3/disk"

	"github.com/stretchr/testify/assert"
)

func TestDeviceRegexp(t *testing.T) {
	tests := []struct {
		DeviceFull string
		DeviceKey  string
	}{
		{"/dev/sda1", "sda1"},
		{"/dev/xda2", "xda2"},
		{"/dev/vda1", "vda1"},
		{"/dev/sda1/odd", "sda1/odd"},
		{"dev/sad1", "sad1"},
		{"/dev/xvd", "xvd"},
		{"/dev/sda1-3", "sda1-3"},
	}

	for _, test := range tests {
		matches := lvmRegexp.FindStringSubmatch(test.DeviceFull)
		if len(matches) > 0 {
			assert.Equal(t, test.DeviceKey, matches[1])
		}
	}
}

func TestLvmRegex_ExtractsDeviceName(t *testing.T) {
	tests := []struct {
		DeviceFull string
		DeviceKey  string
	}{
		{"/dev/mapper/VolGroup01-LogVol00", "VolGroup01-LogVol00"},
		{"/dev/mapper/vg--stuff", "vg--stuff"},
	}

	for _, test := range tests {
		matches := lvmRegexp.FindStringSubmatch(test.DeviceFull)
		if len(matches) > 0 {
			assert.Equal(t, test.DeviceKey, matches[1])
		}

	}
}

func TestLinuxPartitions(t *testing.T) {
	partitions, err := fetchPartitions(false)
	assert.NoError(t, err)
	assert.NotEmpty(t, partitions)
	for _, partition := range partitions {
		if _, ok := SupportedFileSystems[partition.Fstype]; !ok {
			t.Fatalf("Unsupported file systems are in partitions, fs-type: %s", partition.Fstype)
		}
	}
}

func TestCalculateBytesRate(t *testing.T) {
	lastStats := &LinuxIoCountersStat{
		Name:       "nameeee",
		ReadCount:  uint64(2000),
		WriteCount: uint64(2000),
		ReadBytes:  uint64(2000),
		WriteBytes: uint64(2000),
		ReadTime:   uint64(1000000),
		WriteTime:  uint64(100000),
	}
	counter := &LinuxIoCountersStat{
		Name:       "nameeee",
		ReadCount:  uint64(3000),
		WriteCount: uint64(1000),
		ReadBytes:  uint64(3000),
		WriteBytes: uint64(1000),
		ReadTime:   uint64(1000000),
		WriteTime:  uint64(100000),
	}
	elapsedSeconds := 1.0
	ioSample := CalculateSampleValues(counter, lastStats, 1000)

	expectedReadRate := float64(uint64(3000)-uint64(2000)) / elapsedSeconds
	assert.Equal(t, *ioSample.ReadBytesPerSec, expectedReadRate)
	assert.Equal(t, *ioSample.WriteBytesPerSec, float64(0))

	assert.Equal(t, *ioSample.ReadsPerSec, expectedReadRate)
	assert.Equal(t, *ioSample.WritesPerSec, float64(0))
}

func TestMarshallableSamples(t *testing.T) {
	testCases := []struct {
		elapsedTime int64
		description string
		counter     IOCountersStat
		lastStats   IOCountersStat
	}{
		{
			1000, "normal operation",
			&LinuxIoCountersStat{Name: "n", ReadCount: uint64(3000), WriteCount: uint64(1000), ReadBytes: uint64(3000),
				WriteBytes: uint64(1000), ReadTime: uint64(2000000), WriteTime: uint64(200000)},
			&LinuxIoCountersStat{Name: "n", ReadCount: uint64(2000), WriteCount: uint64(2000), ReadBytes: uint64(2000),
				WriteBytes: uint64(2000), ReadTime: uint64(1000000), WriteTime: uint64(100000)},
		},
		{
			0, "with 0 elapsed time",
			&LinuxIoCountersStat{Name: "n", ReadCount: uint64(3000), WriteCount: uint64(1000), ReadBytes: uint64(3000),
				WriteBytes: uint64(1000), ReadTime: uint64(2000000), WriteTime: uint64(200000)},
			&LinuxIoCountersStat{Name: "n", ReadCount: uint64(2000), WriteCount: uint64(2000), ReadBytes: uint64(2000),
				WriteBytes: uint64(2000), ReadTime: uint64(1000000), WriteTime: uint64(100000)},
		},
		{
			1000, "deltas = 0",
			&LinuxIoCountersStat{Name: "n", ReadCount: uint64(3000), WriteCount: uint64(1000), ReadBytes: uint64(3000),
				WriteBytes: uint64(1000), ReadTime: uint64(1000000), WriteTime: uint64(100000)},
			&LinuxIoCountersStat{Name: "n", ReadCount: uint64(3000), WriteCount: uint64(1000), ReadBytes: uint64(3000),
				WriteBytes: uint64(1000), ReadTime: uint64(1000000), WriteTime: uint64(100000)},
		},
		{0, "all zero'ed", &LinuxIoCountersStat{Name: "n"}, &LinuxIoCountersStat{Name: "n"}},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			ioSample := CalculateSampleValues(tc.counter, tc.lastStats, tc.elapsedTime)

			_, err := json.Marshal(ioSample)
			assert.NoError(t, err)
		})
	}
}

func TestDiskIOCounters(t *testing.T) {
	ret, err := fetchIoCounters()
	assert.NoError(t, err)
	assert.NotEqual(t, len(ret), 0)

	empty := LinuxIoCountersStat{}
	for _, io := range ret {
		pio := io.(*LinuxIoCountersStat)
		assert.NotEqual(t, *pio, empty)
	}
}

func TestParseMtab(t *testing.T) {

	var lines = []string{
		"/dev/mapper/VolGroup00-LogVol00 / ext3 rw 0 0",
		"/dev/mapper/VolGroup00-LogVol01 /home ext4 rw 0 0",
		"/dev/sda1 /boot ext3 rw 0 0"}
	var expectedMountInfoStats = []MountInfoStat{
		{
			Device:      "/dev/mapper/VolGroup00-LogVol00",
			MountSource: "/dev/mapper/VolGroup00-LogVol00",
			MountPoint:  "/",
			FSType:      "ext3",
			Opts:        "rw",
		},
		{
			Device:      "/dev/mapper/VolGroup00-LogVol01",
			MountSource: "/dev/mapper/VolGroup00-LogVol01",
			MountPoint:  "/home",
			FSType:      "ext4",
			Opts:        "rw",
		},
		{
			Device:      "/dev/sda1",
			MountSource: "/dev/sda1",
			MountPoint:  "/boot",
			FSType:      "ext3",
			Opts:        "rw",
		},
	}

	for i, line := range lines {
		mi, _ := parseMtab(line)
		assert.Equal(t, expectedMountInfoStats[i], mi)
	}
}

/*
36 35 98:0 /mnt1 /mnt2 rw,noatime master:1 - ext3 /dev/root rw,errors=continue
(1)(2)(3)   (4)   (5)      (6)      (7)   (8) (9)   (10)         (11)

(1) mount ID:  unique identifier of the mount (may be reused after umount)
(2) parent ID:  ID of parent (or of self for the top of the mount tree)
(3) major:minor:  value of st_dev for files on filesystem
(4) root:  root of the mount within the filesystem
(5) mount point:  mount point relative to the process's root
(6) mount options:  per mount options
(7) optional fields:  zero or more fields of the form "tag[:value]"
(8) separator:  marks the end of the optional fields
(9) filesystem type:  name of filesystem of the form "type[.subtype]"
(10) mount source:  filesystem specific information or "none"
(11) super options:  per super block options
*/
func TestParseMountinfo(t *testing.T) {

	// next 2 lines are standard devs
	// last line is lvm dev
	var lines = []string{
		"39 0 8:3 / / rw,relatime shared:1 - xfs /dev/sda3 rw,attr2,inode64,logbsize=256k,sunit=512,swidth=512,noquota",
		"45 39 8:1 / /boot rw,relatime shared:28 - xfs /dev/sda1 rw,attr2,inode64,logbsize=256k,sunit=512,swidth=512,noquota",
		"46 39 253:0 / /notes rw,relatime shared:29 - xfs /dev/mapper/vg--stuff rw,attr2,inode64,sunit=512,swidth=512,noquota",
		"51 39 253:0 / /notes rw,relatime shared:29 - xfs /dev/mapper/vg--stuff ro,attr2,inode64,sunit=512,swidth=512,noquota",
		"62 39 253:0 / /notes ro,relatime shared:29 - xfs /dev/mapper/vg--stuff1 rw",
	}
	var expectedMountInfoStats = []MountInfoStat{
		{
			mountID:     39,
			parentID:    0,
			Device:      "/dev/sda3",
			MountPoint:  "/",
			Root:        "/",
			MajMin:      "8:3",
			FSType:      "xfs",
			MountSource: "/dev/sda3",
			Opts:        "rw,relatime,rw",
		},
		{
			mountID:     45,
			parentID:    39,
			Device:      "/dev/sda1",
			MountPoint:  "/boot",
			Root:        "/",
			MajMin:      "8:1",
			FSType:      "xfs",
			MountSource: "/dev/sda1",
			Opts:        "rw,relatime,rw",
		},
		{
			mountID:  46,
			parentID: 39,
			//we return the full name instead of dm-0 (that mapping should be done in CalculateDeviceMappings)
			Device:      "/dev/mapper/vg--stuff",
			MountPoint:  "/notes",
			Root:        "/",
			MajMin:      "253:0",
			FSType:      "xfs",
			MountSource: "/dev/mapper/vg--stuff",
			Opts:        "rw,relatime,rw",
		},
		{
			mountID:  51,
			parentID: 39,
			//we return the full name instead of dm-0 (that mapping should be done in CalculateDeviceMappings)
			Device:      "/dev/mapper/vg--stuff",
			MountPoint:  "/notes",
			Root:        "/",
			MajMin:      "253:0",
			FSType:      "xfs",
			MountSource: "/dev/mapper/vg--stuff",
			Opts:        "rw,relatime,ro",
		},
		{
			mountID:  62,
			parentID: 39,
			//we return the full name instead of dm-0 (that mapping should be done in CalculateDeviceMappings)
			Device:      "/dev/mapper/vg--stuff1",
			MountPoint:  "/notes",
			Root:        "/",
			MajMin:      "253:0",
			FSType:      "xfs",
			MountSource: "/dev/mapper/vg--stuff1",
			Opts:        "ro,relatime,rw",
		},
	}

	for i, line := range lines {
		mi, _ := parseMountInfo(line)
		assert.Equal(t, expectedMountInfoStats[i], mi)
	}
}

func TestIsRootFs(t *testing.T) {
	var rootFSTest = []struct {
		name string
		in   string
		out  bool
	}{
		{"Default rootfs name", "/dev/root", true},
		{"sda partition", "/dev/sda1", false},
		{"ssd partition", "/dev/nvme0n1p1", false},
	}
	for _, tt := range rootFSTest {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.out, isRootFS(tt.in))
		})
	}
}

func TestPartitionsInfo(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "tmpproc")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	err = ioutil.WriteFile(filepath.Join(tmpDir, partitions), []byte("major minor  #blocks  name\n\n   7        7      44308 loop7\n 259        0    8388608 nvme0n1\n 259        1    8387567 nvme0n1p1\n 259        2    8388608 nvme1n1"), 0666)
	assert.NoError(t, err)
	var expectedDevices = []BlockDevice{
		{
			Major:  "7",
			Minor:  "7",
			blocks: 44308,
			Name:   "loop7",
		},
		{
			Major:  "259",
			Minor:  "0",
			blocks: 8388608,
			Name:   "nvme0n1",
		},
		{
			Major:  "259",
			Minor:  "1",
			blocks: 8387567,
			Name:   "nvme0n1p1",
		},
		{
			Major:  "259",
			Minor:  "2",
			blocks: 8388608,
			Name:   "nvme1n1",
		},
	}
	os.Setenv("HOST_PROC", tmpDir)
	devices := partitionsInfo()
	os.Unsetenv("HOST_PROC")
	assert.Equal(t, expectedDevices, devices)
}

func TestParsePartition(t *testing.T) {
	var partitionsFileTest = []struct {
		name string
		in   string
		out  BlockDevice
	}{
		{"Normal entry for nvme0n1p1 partition", "259        1    8387567 nvme0n1p1", BlockDevice{"259", "1", 8387567, "nvme0n1p1"}},
		{"Normal entry for loop device", "7        8      33076 loop8", BlockDevice{"7", "8", 33076, "loop8"}},
	}
	for _, tt := range partitionsFileTest {
		t.Run(tt.name, func(t *testing.T) {
			b, err := parsePartitions(tt.in)
			assert.NoError(t, err)
			assert.Equal(t, tt.out, b)
		})
	}
}

type MockStorageSampleWrapper struct {
	partitions []PartitionStat
}

var partitionStats = []PartitionStat{
	{
		Device:     "/dev/sda1",
		Mountpoint: "/",
		Fstype:     "ext4",
		Opts:       "",
	},
	{
		Device:     "/dev/sda2",
		Mountpoint: "/boot",
		Fstype:     "ext2",
		Opts:       "",
	},
	{
		Device:     "/dev/sda1",
		Mountpoint: "/temp",
		Fstype:     "ext4",
		Opts:       "",
	},
	{
		Device:     "/dev/mapper/vg-data",
		Mountpoint: "/mnt/data",
		Fstype:     "ext4",
		Opts:       "",
	},
}

var usageTotal1 = uint64(23532463)
var usageFree1 = uint64(13232523)
var usageTotal2 = uint64(123456789)
var usageFree2 = uint64(12345)

func (s *MockStorageSampleWrapper) Partitions() ([]PartitionStat, error) {
	return s.partitions, nil
}

func (s *MockStorageSampleWrapper) Usage(path string) (*disk.UsageStat, error) {
	switch path {
	case partitionStats[0].Mountpoint:
		return &disk.UsageStat{
			Total: usageTotal1,
			Free:  usageFree1,
			Used:  usageTotal1 - usageFree1,
		}, nil
	case partitionStats[1].Mountpoint:
		return &disk.UsageStat{
			Total: usageTotal2,
			Free:  usageFree2,
			Used:  usageTotal2 - usageFree2,
		}, nil
	case partitionStats[2].Mountpoint:
		return &disk.UsageStat{
			Total: usageTotal1,
			Free:  usageFree1,
			Used:  usageTotal1 - usageFree1,
		}, nil
	}
	return nil, nil
}

func (s *MockStorageSampleWrapper) IOCounters() (map[string]IOCountersStat, error) {
	return nil, nil
}

func (s *MockStorageSampleWrapper) CalculateSampleValues(counter, lastStats IOCountersStat, elapsedMs int64) *Sample {
	return nil
}

func TestIgnoredDevice(t *testing.T) {
	ctx := new(mocks.AgentContext)
	ctx.On("Config").Return(&config.Config{
		FileDevicesIgnored: []string{"sda2", "vg-data"},
	})

	ss := NewSampler(ctx)
	assert.NotNil(t, ss)
	ss.storageUtilities = &MockStorageSampleWrapper{partitions: partitionStats}

	results, err := ss.Sample()

	// then we get 2 results (2 mountpoints for the same dev)
	assert.NoError(t, err)
	assert.NotNil(t, results)
	assert.NotNil(t, ss.Samples)
	assert.Len(t, results, 2)
	assert.EqualValues(t, results, ss.lastSamples)

	sample, ok := results[0].(*Sample)
	assert.True(t, ok)
	assert.EqualValues(t, partitionStats[0].Fstype, sample.FileSystemType)
	assert.EqualValues(t, partitionStats[0].Mountpoint, sample.MountPoint)
	assert.EqualValues(t, partitionStats[0].Device, sample.Device)
	assert.EqualValues(t, "false", sample.IsReadOnly)
	assert.EqualValues(t, usageTotal1-usageFree1, *sample.UsedBytes)
	assert.EqualValues(t, usageTotal1, *sample.TotalBytes)
	assert.EqualValues(t, usageFree1, *sample.FreeBytes)

	sample, ok = results[1].(*Sample)
	assert.True(t, ok)
	assert.EqualValues(t, partitionStats[2].Fstype, sample.FileSystemType)
	assert.EqualValues(t, partitionStats[2].Mountpoint, sample.MountPoint)
	assert.EqualValues(t, partitionStats[2].Device, sample.Device)
	assert.EqualValues(t, "false", sample.IsReadOnly)
	assert.EqualValues(t, usageTotal1-usageFree1, *sample.UsedBytes)
	assert.EqualValues(t, usageTotal1, *sample.TotalBytes)
	assert.EqualValues(t, usageFree1, *sample.FreeBytes)
}
