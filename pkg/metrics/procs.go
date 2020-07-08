// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metrics

import (
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/types"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/process"
	"github.com/sirupsen/logrus"
)

var pslog = log.WithFieldsF(func() logrus.Fields {
	return logrus.Fields{
		"component": "Metrics",
		"sampler":   "ProcessSampler",
	}
})

func NewProcessSample(pid int32) *types.ProcessSample {
	return &types.ProcessSample{
		ProcessID: pid,
		Contained: "false",
	}
}

// Deprecated (just for windows compatibility before we migrate it to the "process" package)
type ProcessWrapper interface {
	Ppid() (int32, error)
}

// Deprecated (just for windows compatibility before we migrate it to the "process" package)
type ProcessCacheEntry struct {
	process    ProcessWrapper
	lastCPU    *cpu.TimesStat
	lastSample *types.ProcessSample // The last event we generated for this process, so we can re-use metadata which doesn't change
}

// Deprecated (just for windows compatibility before we migrate it to the "process" package)
type ProcessInterrogator interface {
	NewProcess(int32) (ProcessWrapper, error)
}

type InternalProcess struct {
	process *process.Process
}

func (pw *InternalProcess) Ppid() (int32, error) {
	return pw.process.Ppid()
}

func NewInternalProcess(p *process.Process) *InternalProcess {
	return &InternalProcess{
		process: p,
	}
}

type InternalProcessInterrogator struct {
	privileged bool
}

func NewInternalProcessInterrogator(privileged bool) *InternalProcessInterrogator {
	return &InternalProcessInterrogator{
		privileged: privileged,
	}
}

func (ip *InternalProcessInterrogator) NewProcess(pid int32) (ProcessWrapper, error) {
	process, err := process.NewProcess(pid)
	return NewInternalProcess(process), err
}
