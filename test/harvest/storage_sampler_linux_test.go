// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux && harvest
// +build linux,harvest

package harvest

import (
	"io/ioutil"
	"math/rand"
	"os"
	"regexp"
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/metrics/storage"
	"github.com/newrelic/infrastructure-agent/pkg/sample"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageSample(t *testing.T) {
	du, err := disk.Usage("/")
	if err != nil {
		t.Skipf("this linux distro can't get the usage statistics: %v", err.Error())
	}
	if du.InodesTotal == 0 {
		t.Skipf("this linux distro is not supported for inodes: %#v", du)
	}

	// GIVEN a Storage Sampler
	ps := storage.NewSampler(contextMock())

	// THAT has already sampled values in the past
	_, err = ps.Sample()
	require.NoError(t, err)

	// AND there are write operations to a file
	file, err := ioutil.TempFile("/tmp", "prefix")
	require.NoError(t, err)
	defer os.Remove(file.Name())
	//write 1Mb to the file
	data := make([]byte, 1024*1024)
	rand.Read(data)
	file.Write(data)

	// WHEN it samples again
	samples, err := ps.Sample()
	require.NoError(t, err)

	// THEN the read samples are of the correct type, with a valid format and non-zero values for those
	// metrics that can't be zero
	ss := fullSample(t, samples)

	assert.Equal(t, "StorageSample", ss.EventType)

	assert.NotEmpty(t, ss.MountPoint)
	assert.NotEmpty(t, ss.Device)
	assert.NotEmpty(t, ss.FileSystemType)
	assert.NotEmpty(t, ss.Device)
	assert.NotEmpty(t, ss.IsReadOnly)

	require.NotNil(t, ss.InodesTotal)
	require.NotNil(t, ss.InodesFree)
	require.NotNil(t, ss.InodesUsed)
	require.NotNil(t, ss.InodesUsedPercent)
	assert.NotZero(t, *ss.InodesTotal)
	assert.NotZero(t, *ss.InodesFree)
	assert.NotZero(t, *ss.InodesUsed)
	assert.NotZero(t, *ss.InodesUsedPercent)
	assert.NotNil(t, ss.TotalBytes)
	assert.NotZero(t, *ss.TotalBytes)
	assert.NotNil(t, ss.UsedBytes)
	assert.NotZero(t, *ss.UsedBytes)

	utilizationSupported, err := distroSupportsUtilization()
	require.NoError(t, err)
	if utilizationSupported {
		assert.NotNil(t, ss.TotalUtilizationPercent)
		assert.NotNil(t, ss.ReadBytesPerSec)
		assert.NotNil(t, ss.WriteBytesPerSec)
		assert.NotNil(t, ss.ReadWriteBytesPerSecond)
		assert.NotNil(t, ss.ReadsPerSec)
		assert.NotNil(t, ss.WritesPerSec)
		assert.NotNil(t, ss.ReadWriteBytesPerSecond)
	}
}

// distroSupportsUtilization will return if the distro supports utilization. Some linux distros (at least in aws)
// do not have proper information about some partitions and only have them for the whole disk. i.e. debian 8.7 :
// cat /proc/diskstats
// 259       0 nvme0n1 8527 0 174006 15004 15873 0 337402 3573496 0 8236 3588496
// 259       1 nvme0n1p1 0 0 0 0 0 0 0 0 0 0 0
// 259       2 nvme0n1p2 0 0 0 0 0 0 0 0 0 0 0
// mount
// /dev/nvme0n1p2 on / type ext4 (rw,relatime,data=ordered)
func distroSupportsUtilization() (bool, error) {
	isDebian8, err := isDebian("8")

	return !isDebian8, err
}

func isDebian(version string) (bool, error) {
	reg, err := regexp.Compile("^" + version)
	if err != nil {
		return false, err
	}
	dat, err := os.ReadFile("/etc/debian_version")
	if reg.Match(dat) {
		return true, nil
	}
	return false, nil
}

// This test assumes that the temporary folder is mounted in the same device as the root directory
func TestStorageSampleInodes(t *testing.T) {
	t.Skip("Flaky test")

	du, err := disk.Usage("/")
	if err != nil {
		t.Skipf("this linux distro can't get the usage statistics: %v", err.Error())
	}
	if du.InodesTotal == 0 {
		t.Skipf("this linux distro is not supported for inodes: %#v", du)
	}

	// GIVEN a Storage Sampler
	ps := storage.NewSampler(contextMock())

	// THAT has already sampled values in the past
	samples, err := ps.Sample()
	require.NoError(t, err)
	s1 := fullSample(t, samples)

	// WHEN the number of inodes in use is increased
	const newInodes = 50
	for i := 0; i < newInodes; i++ {
		f, err := ioutil.TempFile("", "TestStorageSampleInodes")
		require.NoError(t, err)
		_, err = f.Write([]byte("hello!"))
		require.NoError(t, err)
		require.NoError(t, f.Close())
	}

	// THEN the next sample reflects this change
	samples, err = ps.Sample()
	require.NoError(t, err)
	s2 := fullSample(t, samples)

	assert.Truef(t, *s2.InodesUsed-*s1.InodesUsed > uint64(newInodes*0.8),
		"Inodes Used: Expected %v ~= %v + %v (with some tolerance margin)", *s2.InodesUsed, *s1.InodesUsed, newInodes)
	assert.Truef(t, *s1.InodesFree-*s2.InodesFree > uint64(newInodes*0.8),
		"Inodes Free: Expected %v ~= %v + %v (with some tolerance margin)", *s1.InodesFree, *s2.InodesFree, newInodes)
	assert.Truef(t, *s1.InodesUsedPercent < *s2.InodesUsedPercent,
		"Inodes Used %%: Expected %v < %v", *s1.InodesUsedPercent, *s2.InodesUsedPercent)
	assert.Equal(t, *s1.InodesTotal, *s2.InodesTotal)
}

// fullSample returns a sample containing usage data
func fullSample(t *testing.T, samples sample.EventBatch) *storage.Sample {
	t.Helper()
	for i := range samples {
		if s, ok := samples[i].(*storage.Sample); ok {
			if s.InodesTotal != nil && *s.InodesTotal != 0 {
				return s
			}
		}
	}
	require.Failf(t, "can't find valid storage sample", "%#v", samples)
	return nil
}
