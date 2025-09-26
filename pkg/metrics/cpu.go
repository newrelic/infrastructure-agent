// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metrics

import (
	"fmt"
	"runtime/debug"

	"github.com/shirou/gopsutil/v3/cpu"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

//nolint:gochecknoglobals
var (
	cpuLog = log.WithComponent("CPUMonitor")

	// defaultCPUSample is a reusable zero-value CPU sample.
	defaultCPUSample = &CPUSample{
		CPUPercent:       0,
		CPUUserPercent:   0,
		CPUSystemPercent: 0,
		CPUIOWaitPercent: 0,
		CPUIdlePercent:   0,
		CPUStealPercent:  0,
	}
)

type CPUSample struct {
	CPUPercent       float64 `json:"cpuPercent"`
	CPUUserPercent   float64 `json:"cpuUserPercent"`
	CPUSystemPercent float64 `json:"cpuSystemPercent"`
	CPUIOWaitPercent float64 `json:"cpuIOWaitPercent"`
	CPUIdlePercent   float64 `json:"cpuIdlePercent"`
	CPUStealPercent  float64 `json:"cpuStealPercent"`
}

type CPUMonitor struct {
	context  agent.AgentContext
	last     []cpu.TimesStat
	cpuTimes func(bool) ([]cpu.TimesStat, error)
	// Windows-specific monitor using PDH
	windowsMonitor interface {
		sample() (*CPUSample, error)
		close() error
	}
}

// Close releases any resources held by the CPU monitor.
func (m *CPUMonitor) Close() error {
	if m.windowsMonitor != nil {
		return m.windowsMonitor.close()
	}
	return nil
}

func (m *CPUMonitor) Sample() (sample *CPUSample, err error) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = fmt.Errorf("panic in CPUMonitor.Sample: %v\nStack: %s", panicErr, debug.Stack())
		}
	}()

	// Use Windows-specific implementation if available
	if m.windowsMonitor != nil {
		return m.windowsMonitor.sample()
	}

	if m.last == nil {
		m.last, err = m.cpuTimes(false)

		return defaultCPUSample, nil
	}

	currentTimes, err := m.cpuTimes(false)
	helpers.LogStructureDetails(cpuLog, currentTimes, "CpuTimes", "raw", nil)

	// in container envs we might get an empty array and the code panics after this
	if len(currentTimes) <= 0 {
		return defaultCPUSample, nil
	}

	delta := cpuDelta(&currentTimes[0], &m.last[0])

	userDelta := delta.User + delta.Nice
	systemDelta := delta.System + delta.Irq + delta.Softirq
	stolenDelta := delta.Steal

	// Determine percentage values by dividing the total CPU time by each portion, then multiply by 100 to get a percentage from 0-100.
	var userPercent, stolenPercent, systemPercent, ioWaitPercent float64

	// Calculate total manually instead of using deprecated Total() method
	deltaTotal := delta.User + delta.Nice + delta.System + delta.Idle + delta.Iowait + delta.Irq + delta.Softirq + delta.Steal + delta.Guest + delta.GuestNice
	if deltaTotal != 0 {
		userPercent = userDelta / deltaTotal * 100.0
		stolenPercent = stolenDelta / deltaTotal * 100.0
		systemPercent = systemDelta / deltaTotal * 100.0
		ioWaitPercent = delta.Iowait / deltaTotal * 100.0
	}
	idlePercent := 100 - userPercent - systemPercent - ioWaitPercent - stolenPercent

	sample = &CPUSample{
		CPUPercent:       userPercent + systemPercent + ioWaitPercent + stolenPercent,
		CPUUserPercent:   userPercent,
		CPUSystemPercent: systemPercent,
		CPUIOWaitPercent: ioWaitPercent,
		CPUIdlePercent:   idlePercent,
		CPUStealPercent:  stolenPercent,
	}

	// log samples when cpuPercent is < 0
	if sample.CPUPercent < 0 {
		cpuLog.WithField("currentTimes", currentTimes).WithField("lastTimes", m.last).Warn("cpuPercent is lower than zero")
	}

	m.last = currentTimes

	return
}

func cpuDelta(current, previous *cpu.TimesStat) *cpu.TimesStat {
	var result cpu.TimesStat

	result.CPU = current.CPU
	result.Guest = current.Guest - previous.Guest
	result.GuestNice = current.GuestNice - previous.GuestNice
	result.Idle = current.Idle - previous.Idle
	result.Iowait = current.Iowait - previous.Iowait
	result.Irq = current.Irq - previous.Irq
	result.Nice = current.Nice - previous.Nice
	result.Softirq = current.Softirq - previous.Softirq

	result.Steal = current.Steal - previous.Steal
	// Fixes a bug in some paravirtualized environments that caused steal time decreasing during migrations
	// https://0xstubs.org/debugging-a-flaky-cpu-steal-time-counter-on-a-paravirtualized-xen-guest/
	// https://bugs.debian.org/cgi-bin/bugreport.cgi?bug=871608
	// https://lkml.org/lkml/2017/10/10/182
	if result.Steal < 0 {
		result.Steal = 0
	}
	result.System = current.System - previous.System
	result.User = current.User - previous.User
	return &result
}
