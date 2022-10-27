// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux && harvest
// +build linux,harvest

package harvest

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"syscall"
	"testing"

	"github.com/shirou/gopsutil/v3/disk"

	"github.com/newrelic/infrastructure-agent/internal/agent/mocks"
	"github.com/newrelic/infrastructure-agent/internal/testhelpers"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/metrics"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHostSharedMemory(t *testing.T) {
	ctx := new(mocks.AgentContext)
	ctx.On("Config").Return(&config.Config{
		MetricsNetworkSampleRate: 1,
	})
	storageSampler := storage.NewSampler(ctx)

	systemSampler := metrics.NewSystemSampler(ctx, storageSampler, nil)

	sampleB, _ := systemSampler.Sample()
	beforeSample := sampleB[0].(*metrics.SystemSample)

	f, err := os.Create("/dev/shm/test")
	require.NoError(t, err)

	for i := 0; i < 1024*1024; i++ {
		f.Write([]byte("0"))
	}

	f.Sync()
	defer func() {
		require.NoError(t, f.Close())
		os.Remove(f.Name())
	}()

	testhelpers.Eventually(t, timeout, func(st require.TestingT) {
		sampleB, _ = systemSampler.Sample()
		afterSample := sampleB[0].(*metrics.SystemSample)

		assert.True(st, afterSample.MemorySharedBytes >= beforeSample.MemorySharedBytes+(1024*1024), "Shared Memory used did not increase enough, SharedMemoryBefore: %f SharedMemoryAfter %f ", beforeSample.MemorySharedBytes, afterSample.MemorySharedBytes)
	})
}

func TestHostCachedMemory(t *testing.T) {
	ctx := new(mocks.AgentContext)
	ctx.On("Config").Return(&config.Config{
		MetricsNetworkSampleRate: 1,
	})
	storageSampler := storage.NewSampler(ctx)

	systemSampler := metrics.NewSystemSampler(ctx, storageSampler, nil)

	sampleB, _ := systemSampler.Sample()
	beforeSample := sampleB[0].(*metrics.SystemSample)

	f, err := ioutil.TempFile("/tmp", "")
	assert.NoError(t, err)
	defer os.Remove(f.Name())

	// Force memory spike
	for i := 0; i < 1e5; i++ {
		f.Write([]byte("00000000000000000000"))
	}

	f.Sync()
	f.Close()

	_, err = ioutil.ReadFile(f.Name())

	assert.NoError(t, err)

	testhelpers.Eventually(t, timeout, func(st require.TestingT) {
		sampleB, _ = systemSampler.Sample()
		afterSample := sampleB[0].(*metrics.SystemSample)

		expectedIncreaseBytes := 500000.0
		assert.True(st, beforeSample.MemoryCachedBytes+expectedIncreaseBytes <= afterSample.MemoryCachedBytes, "CachedMemory used did not increase enough, expected an increase by %f CachedMemoryBefore: %f CachedMemoryAfter %f ", expectedIncreaseBytes, beforeSample.MemoryCachedBytes, afterSample.MemoryCachedBytes)
	})
}

func TestHostDisk(t *testing.T) {
	ctx := new(mocks.AgentContext)
	ctx.On("Config").Return(&config.Config{
		MetricsNetworkSampleRate: 1,
	})
	storageSampler := storage.NewSampler(ctx)
	storageSamples, _ := storageSampler.Sample()

	storageSample := &storage.Sample{}
	storageSample = storageSamples[0].(*storage.Sample)
	t.Logf("DISK:: %s", storageSample.MountPoint)
	t.Logf("Free Bytes sampleB: %v", *storageSample.FreeBytes)
	t.Logf("Used Bytes sampleB: %v", *storageSample.UsedBytes)

	diskMonitor := metrics.NewDiskMonitor(storageSampler)
	diskMonitor.Sample()
	// Create a file
	f, err := os.Create(pathForTmpFile())
	require.NoError(t, err, "Error creating a file.")
	data := make([]byte, int(1e5), int(1e5)) // Initialize an empty byte slice
	f.Write(data)                            // Write it to the file
	defer f.Close()
	defer func() {
		os.Remove(pathForTmpFile())
	}()
	// Read file
	ioutil.ReadFile(pathForTmpFile())

	storageSamples, _ = storageSampler.Sample()
	sampleA, _ := diskMonitor.Sample()
	t.Logf("Free Bytes sampleA: %v", sampleA.FreeBytes)
	t.Logf("User Bytes sampleA: %v", sampleA.UsedBytes)
	t.Logf("Reads per sec sampleA: %v", sampleA.ReadsPerSec)
	t.Logf("Writes per sec sampleA: %v", sampleA.WritesPerSec)

	assert.NotEqual(t, 0, sampleA.ReadsPerSec, "Reads per sec is 0")
	assert.NotEqual(t, 0, sampleA.WritesPerSec, "Writes per sec is 0")
	assert.True(t, *storageSample.UsedBytes+1e4 < sampleA.UsedBytes, "Used bytes did not increase enough, UserBytesBefore: %f UserBytesAfter %f ", *(storageSample.UsedBytes), sampleA.UsedBytes)
}

func TestHostSlabMemory(t *testing.T) {
	ctx := new(mocks.AgentContext)
	ctx.On("Config").Return(&config.Config{
		MetricsNetworkSampleRate: 1,
	})
	storageSampler := storage.NewSampler(ctx)

	systemSampler := metrics.NewSystemSampler(ctx, storageSampler, nil)

	sampleB, _ := systemSampler.Sample()
	beforeSample := sampleB[0].(*metrics.SystemSample)

	for i := 0; i < 1000; i++ {
		cmd := exec.Command("/bin/bash", "-c", "echo x")
		err := cmd.Start()
		require.NoError(t, err)
		defer func() {
			if cmd.Process != nil {
				cmd.Process.Kill()
			}
		}()
	}

	testhelpers.Eventually(t, timeout, func(st require.TestingT) {
		sampleB, _ = systemSampler.Sample()
		afterSample := sampleB[0].(*metrics.SystemSample)

		expectedIncreaseBytes := 500000.0
		assert.True(t, beforeSample.MemorySlabBytes+expectedIncreaseBytes <= afterSample.MemorySlabBytes, "Slab memory used did not increase enough, expected %f increase, SlabMemoryBefore: %f SlabMemoryAfter %f ", expectedIncreaseBytes, beforeSample.MemorySlabBytes, afterSample.MemorySlabBytes)
	})
}

func TestHostBuffersMemory(t *testing.T) {
	ctx := new(mocks.AgentContext)
	ctx.On("Config").Return(&config.Config{
		MetricsNetworkSampleRate: 1,
	})
	storageSampler := storage.NewSampler(ctx)
	systemSampler := metrics.NewSystemSampler(ctx, storageSampler, nil)

	// clear cache
	err := ioutil.WriteFile("/proc/sys/vm/drop_caches", []byte("3"), 0o200)
	require.NoError(t, err)

	// take the first sample to compare with
	sampleB, _ := systemSampler.Sample()
	beforeSample := sampleB[0].(*metrics.SystemSample)

	// reading directly from dist will increase the buffered memory
	// https://tldp.org/LDP/sag/html/buffer-cache.html
	root, err := rootDevice()
	require.NoError(t, err)

	fd, err := syscall.Open(root, syscall.O_RDONLY, 0o777)
	require.NoError(t, err)

	expectedIncreaseBytes := 10 * 1024 * 1024
	buffer := make([]byte, expectedIncreaseBytes, expectedIncreaseBytes)
	_, err = syscall.Read(fd, buffer)
	require.NoError(t, err)

	err = syscall.Close(fd)
	require.NoError(t, err)

	// second sample to compare with
	sampleB, _ = systemSampler.Sample()
	afterSample := sampleB[0].(*metrics.SystemSample)

	assert.True(t, (*beforeSample.MemoryBuffers)+float64(expectedIncreaseBytes) <= *afterSample.MemoryBuffers, "MemoryBuffers used did not increase enough, expected an increase by %f MemoryBuffersBefore: %f MemoryBuffersAfter %f ", expectedIncreaseBytes, beforeSample.MemoryBuffers, afterSample.MemoryBuffers)
}

func rootDevice() (string, error) {
	all := true
	parts, err := disk.PartitionsWithContext(context.Background(), all)
	if err != nil {
		return "", err
	}
	for _, part := range parts {
		if part.Mountpoint == "/" {
			return part.Device, nil
		}
	}
	return "", errors.New("for some reason I could not find the root device 🤷‍")
}
