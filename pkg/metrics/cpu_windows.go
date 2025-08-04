//go:build windows
// +build windows

// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metrics

import (
	"fmt"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	nrwin "github.com/newrelic/infrastructure-agent/internal/windows"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

//nolint:gochecknoglobals
var cpulog = log.WithComponent("CPUWindows")

// Windows CPU performance counter paths
const (
	processorTimeTotal  = "\\Processor(_Total)\\% Processor Time"
	userTimeTotal       = "\\Processor(_Total)\\% User Time"
	privilegedTimeTotal = "\\Processor(_Total)\\% Privileged Time"
	idleTimeTotal       = "\\Processor(_Total)\\% Idle Time"
	interruptTimeTotal  = "\\Processor(_Total)\\% Interrupt Time"
	dpcTimeTotal        = "\\Processor(_Total)\\% DPC Time"
)

type WindowsCPUMonitor struct {
	context agent.AgentContext
	pdh     nrwin.PdhPoll
	started bool
}

// NewCPUMonitor creates a new Windows CPU monitor using PDH performance counters
func NewCPUMonitor(context agent.AgentContext) *CPUMonitor {
	winMonitor := &WindowsCPUMonitor{
		context: context,
	}

	return &CPUMonitor{
		context:        context,
		cpuTimes:       nil,
		windowsMonitor: winMonitor,
	}
}

func (w *WindowsCPUMonitor) initializePDH() error {
	if w.started {
		return nil
	}

	var err error
	w.pdh, err = nrwin.NewPdhPoll(
		cpulog.Debugf,
		processorTimeTotal,
		userTimeTotal,
		privilegedTimeTotal,
		idleTimeTotal,
		interruptTimeTotal,
		dpcTimeTotal,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize PDH for CPU monitoring: %w", err)
	}

	w.started = true
	return nil
}

func (w *WindowsCPUMonitor) sample() (*CPUSample, error) {
	if err := w.initializePDH(); err != nil {
		return nil, err
	}

	values, err := w.pdh.Poll()
	if err != nil {
		return nil, fmt.Errorf("failed to poll CPU performance counters: %w", err)
	}

	helpers.LogStructureDetails(cpulog, values, "CpuPerfCounters", "raw", nil)

	// Calculate percentages from performance counters
	processorTime := values[processorTimeTotal]
	userTime := values[userTimeTotal]
	privilegedTime := values[privilegedTimeTotal]
	idleTime := values[idleTimeTotal]
	interruptTime := values[interruptTimeTotal]
	dpcTime := values[dpcTimeTotal]

	// Calculate system time as privileged time + interrupt time + DPC time
	systemTime := privilegedTime + interruptTime + dpcTime

	// Ensure we don't exceed 100% due to measurement variations
	if processorTime > 100 {
		processorTime = 100
	}
	if userTime > 100 {
		userTime = 100
	}
	if systemTime > 100 {
		systemTime = 100
	}
	if idleTime > 100 {
		idleTime = 100
	}

	sample := &CPUSample{
		CPUPercent:       processorTime,
		CPUUserPercent:   userTime,
		CPUSystemPercent: systemTime,
		CPUIOWaitPercent: 0, // Windows doesn't have a direct equivalent to IOWait
		CPUIdlePercent:   idleTime,
		CPUStealPercent:  0, // Windows doesn't have steal time (virtualization concept)
	}

	// Log warning if CPU percent is negative (shouldn't happen with PDH)
	if sample.CPUPercent < 0 {
		cpulog.WithField("values", values).Warn("cpuPercent is lower than zero")
	}

	return sample, nil
}

func (w *WindowsCPUMonitor) close() error {
	if w.started {
		return w.pdh.Close()
	}
	return nil
}
