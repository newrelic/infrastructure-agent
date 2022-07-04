// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux && harvest
// +build linux,harvest

package harvest

import (
	"github.com/newrelic/infrastructure-agent/internal/agent/mocks"
	"github.com/newrelic/infrastructure-agent/internal/testhelpers"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/metrics"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"os/exec"
	"testing"
)

func TestHostSharedMemory(t *testing.T) {
	ctx := new(mocks.AgentContext)
	ctx.On("Config").Return(&config.Config{
		MetricsNetworkSampleRate: 1,
	})
	storageSampler := storage.NewSampler(ctx)

	systemSampler := metrics.NewSystemSampler(ctx, storageSampler)

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

	systemSampler := metrics.NewSystemSampler(ctx, storageSampler)

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

	systemSampler := metrics.NewSystemSampler(ctx, storageSampler)

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
