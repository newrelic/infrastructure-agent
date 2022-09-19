// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metrics

import (
	context2 "context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/agent/instrumentation"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/storage"
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
	*HostSample
}

type SystemSampler struct {
	CpuMonitor     *CPUMonitor
	DiskMonitor    *DiskMonitor
	LoadMonitor    *LoadMonitor
	MemoryMonitor  *MemoryMonitor
	HostMonitor    *HostMonitor
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
		HostMonitor:    NewHostMonitor(),
		context:        context,
		waitForCleanup: &sync.WaitGroup{},
	}
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
	ctx := context2.Background()
	// Example of detailed sampler. Having the context as param to Sample(ctx context.Context) would allow
	// us check for existing transaction and reuse it instead of creating new one.
	ctx, trx := instrumentation.SelfInstrumentation.StartTransaction(ctx, "system-sampler-detailed")
	defer trx.End()

	sysSample := &SystemSample{}
	sysSample.Type("SystemSample")

	// Collect CPU
	ctx, seg := trx.StartSegment(ctx, "cpu sample")
	if cpuSample, err := s.CpuMonitor.Sample(); err != nil {
		return nil, err
	} else {
		sysSample.CPUSample = cpuSample
	}
	seg.End()

	ctx, seg = trx.StartSegment(ctx, "disk sample")
	if diskSample, err := s.DiskMonitor.Sample(); err != nil {
		return nil, err
	} else {
		sysSample.DiskSample = diskSample
	}
	seg.End()

	ctx, seg = trx.StartSegment(ctx, "load sample")
	if loadSample, err := s.LoadMonitor.Sample(); err != nil {
		return nil, err
	} else {
		sysSample.LoadSample = loadSample
	}
	seg.End()

	ctx, seg = trx.StartSegment(ctx, "memory sample")
	if memorySample, err := s.MemoryMonitor.Sample(); err != nil {
		return nil, err
	} else {
		sysSample.MemorySample = memorySample
	}
	seg.End()

	ctx, seg = trx.StartSegment(ctx, "host sample")
	if hostSample, err := s.HostMonitor.Sample(); err != nil {
		return nil, err
	} else {
		sysSample.HostSample = hostSample
	}
	seg.End()

	helpers.LogStructureDetails(syslog, sysSample, "SystemSample", "final", nil)
	results = append(results, sysSample)

	return
}
