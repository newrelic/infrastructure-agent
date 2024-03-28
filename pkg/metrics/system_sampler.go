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
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/hostid"
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
	HostID string `json:"host.id,omitempty"`
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
	hostIDProvider hostid.Provider
}

func NewSystemSampler(context agent.AgentContext, storageSampler *storage.Sampler, ntpMonitor NtpMonitor, hostIDProvider hostid.Provider) *SystemSampler {
	cfg := context.Config()
	return &SystemSampler{
		CpuMonitor:     NewCPUMonitor(context),
		DiskMonitor:    NewDiskMonitor(storageSampler),
		LoadMonitor:    NewLoadMonitor(),
		MemoryMonitor:  NewMemoryMonitor(cfg.IgnoreReclaimable),
		HostMonitor:    NewHostMonitor(ntpMonitor),
		context:        context,
		waitForCleanup: &sync.WaitGroup{},
		hostIDProvider: hostIDProvider,
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

	cpuSample, err := s.CpuMonitor.Sample()
	if err != nil {
		seg.End()

		return nil, err
	}

	sysSample.CPUSample = cpuSample
	seg.End()

	// Collect Disk
	ctx, seg = trx.StartSegment(ctx, "disk sample")

	diskSample, err := s.DiskMonitor.Sample()
	if err != nil {
		seg.End()

		return nil, err
	}

	sysSample.DiskSample = diskSample
	seg.End()

	// Collect Load
	ctx, seg = trx.StartSegment(ctx, "load sample")

	loadSample, err := s.LoadMonitor.Sample()
	if err != nil {
		seg.End()

		return nil, err
	}

	sysSample.LoadSample = loadSample
	seg.End()

	// Collect Memory
	ctx, seg = trx.StartSegment(ctx, "memory sample")

	memorySample, err := s.MemoryMonitor.Sample()
	if err != nil {
		seg.End()

		return nil, err
	}

	sysSample.MemorySample = memorySample

	seg.End()

	// Collect Host
	_, seg = trx.StartSegment(ctx, "host sample")

	hostSample, err := s.HostMonitor.Sample()
	if err != nil {
		seg.End()

		return nil, err
	}

	sysSample.HostSample = hostSample
	seg.End()

	hostID, hostIDErr := s.hostIDProvider.Provide()
	if hostIDErr != nil {
		syslog.WithError(hostIDErr).Error("cannot retrieve host_id")
	} else {
		sysSample.HostID = hostID
	}

	helpers.LogStructureDetails(syslog, sysSample, "SystemSample", "final", nil)
	results = append(results, sysSample)

	return
}
