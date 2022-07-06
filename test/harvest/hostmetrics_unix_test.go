// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build (linux || darwin) && harvest
// +build linux darwin
// +build harvest

package harvest

import (
	"fmt"
	"github.com/newrelic/infrastructure-agent/internal/agent/mocks"
	"github.com/newrelic/infrastructure-agent/internal/testhelpers"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/metrics"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"runtime"
	"testing"
	"time"
)

const timeout = 5 * time.Second

func TestHostCPU(t *testing.T) {
	ctx := new(mocks.AgentContext)
	ctx.On("Config").Return(&config.Config{
		MetricsNetworkSampleRate: 1,
	})
	storageSampler := storage.NewSampler(ctx)
	systemSampler := metrics.NewSystemSampler(ctx, storageSampler)

	sampleB, _ := systemSampler.Sample()
	beforeSample := sampleB[0].(*metrics.SystemSample)

	quit := make(chan bool)
	// Start a process to spike CPU usage
	go func() {
		i := 0
		for {
			select {
			case <-quit:
				return
			default:
				i++
			}
		}
	}()

	defer func() {
		// Quit goroutine
		quit <- true
	}()

	testhelpers.Eventually(t, timeout, func(st require.TestingT) {
		sampleB, _ = systemSampler.Sample()
		afterSample := sampleB[0].(*metrics.SystemSample)

		assert.True(st, beforeSample.CPUPercent+10 < afterSample.CPUPercent, "CPU did not increase enough, CPUBefore: %f CPUAfter %f ", beforeSample.CPUPercent, afterSample.CPUPercent)

		t.Logf("CPUPercents: %f, %f", beforeSample.CPUPercent, afterSample.CPUPercent)
	})
}

func TestHostMemory(t *testing.T) {
	ctx := new(mocks.AgentContext)
	ctx.On("Config").Return(&config.Config{
		MetricsNetworkSampleRate: 1,
	})
	storageSampler := storage.NewSampler(ctx)

	systemSampler := metrics.NewSystemSampler(ctx, storageSampler)

	sampleB, _ := systemSampler.Sample()
	beforeSample := sampleB[0].(*metrics.SystemSample)

	// Start a process to spike Memory usage
	quit := make(chan bool)
	go func() {
		var s []int
		// Force memory spike
		for i := 0; i < 1e7; i++ {
			s = append(s, i)
			i++
		}
		for {
			select {
			case <-quit:
				return
			default:
			}
		}
	}()

	defer func() {
		quit <- true
	}()

	testhelpers.Eventually(t, timeout, func(st require.TestingT) {
		sampleB, _ = systemSampler.Sample()
		afterSample := sampleB[0].(*metrics.SystemSample)

		assert.True(st, beforeSample.MemoryUsed+1e5 < afterSample.MemoryUsed, "Memory used did not increase enough, MemoryBefore: %f MemoryAfter %f ", beforeSample.MemoryUsed, afterSample.MemoryUsed)

		t.Logf("Memory: %f, %f", beforeSample.MemoryUsed, afterSample.MemoryUsed)
	})
}

func TestHostSwap(t *testing.T) {
	ctx := new(mocks.AgentContext)
	ctx.On("Config").Return(&config.Config{
		MetricsNetworkSampleRate: 1,
	})
	storageSampler := storage.NewSampler(ctx)

	systemSampler := metrics.NewSystemSampler(ctx, storageSampler)

	sampleB, _ := systemSampler.Sample()
	sample := sampleB[0].(*metrics.SystemSample)

	assert.NotNil(t, sample.SwapUsed, "Swap is null")

	t.Logf("Swap Memory: %f", sample.SwapUsed)
}

func pathForTmpFile() (path string) {
	if runtime.GOOS == "linux" {
		path = fmt.Sprintf("%s%s", "/var/tmp/", "testfile") // /tmp is mounted as tmpfs in some systems and cannot be used for disk stats
	} else if runtime.GOOS == "windows" {
		path = fmt.Sprintf("%s\\%s", os.Getenv("%Temp%"), "testfile")
	} else {
		panic("Runtime not supported")
	}
	return
}
