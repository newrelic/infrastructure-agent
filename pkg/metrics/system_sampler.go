// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metrics

import (
	"fmt"

	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/storage"

	"runtime/debug"
	"sync"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/newrelic/infrastructure-agent/pkg/sample"
)

var syslog = log.WithComponent("SystemSampler")

// SystemSample uses pointers to embedded structs to ensure that those attribute
// are only present if they are successfully collected.
type SystemSample struct {
	sample.BaseEvent
	*CPUSample
	*LoadSample
	*MemorySample
	*DiskSample
}

type SystemSampler struct {
	CpuMonitor     *CPUMonitor
	DiskMonitor    *DiskMonitor
	LoadMonitor    *LoadMonitor
	MemoryMonitor  *MemoryMonitor
	context        agent.AgentContext
	stopChannel    chan bool
	waitForCleanup *sync.WaitGroup
}

func NewSystemSampler(context agent.AgentContext, storageSampler *storage.Sampler) *SystemSampler {
	cfg := context.Config()
	return &SystemSampler{
		CpuMonitor:     NewCPUMonitor(context),
		DiskMonitor:    NewDiskMonitor(storageSampler),
		LoadMonitor:    NewLoadMonitor(),
		MemoryMonitor:  NewMemoryMonitor(cfg.IgnoreReclaimable),
		context:        context,
		waitForCleanup: &sync.WaitGroup{},
	}
}

func (s *SystemSampler) Debug() bool {
	if s.context == nil {
		return false
	}
	return s.context.Config().Debug
}

func (s *SystemSampler) sampleInterval() int {
	if s.context != nil {
		return s.context.Config().MetricsSystemSampleRate
	}
	return config.FREQ_INTERVAL_FLOOR_SYSTEM_METRICS
}

func (s *SystemSampler) Interval() time.Duration {
	return time.Second * time.Duration(s.sampleInterval())
}

func (s *SystemSampler) Name() string { return "SystemSampler" }

func (s *SystemSampler) OnStartup() {}

func (s *SystemSampler) Disabled() bool {
	return s.Interval() <= config.FREQ_DISABLE_SAMPLING
}

func (s *SystemSampler) Sample() (results sample.EventBatch, err error) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = fmt.Errorf("Panic in SystemSampler.Sample: %v\nStack: %s", panicErr, debug.Stack())
		}
	}()

	sample := &SystemSample{}
	sample.Type("SystemSample")

	// Collect CPU
	if cpuSample, err := s.CpuMonitor.Sample(); err != nil {
		return nil, err
	} else {
		sample.CPUSample = cpuSample
	}

	if diskSample, err := s.DiskMonitor.Sample(); err != nil {
		return nil, err
	} else {
		sample.DiskSample = diskSample
	}

	if loadSample, err := s.LoadMonitor.Sample(); err != nil {
		return nil, err
	} else {
		sample.LoadSample = loadSample
	}

	if memorySample, err := s.MemoryMonitor.Sample(); err != nil {
		return nil, err
	} else {
		sample.MemorySample = memorySample
	}

	if s.Debug() {
		helpers.LogStructureDetails(syslog, sample, "SystemSample", "final", nil)
	}
	results = append(results, sample)
	return
}
