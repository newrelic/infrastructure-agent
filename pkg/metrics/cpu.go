// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metrics

import (
	"fmt"
	"runtime/debug"

	"github.com/shirou/gopsutil/cpu"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
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
}

func NewCPUMonitor(context agent.AgentContext) *CPUMonitor {
	return &CPUMonitor{context: context, cpuTimes: cpu.Times}
}

func (self *CPUMonitor) Debug() bool {
	if self.context == nil {
		return false
	}
	return self.context.Config().Debug
}

func (self *CPUMonitor) Sample() (sample *CPUSample, err error) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = fmt.Errorf("Panic in CPUMonitor.Sample: %v\nStack: %s", panicErr, debug.Stack())
		}
	}()

	if self.last == nil {
		self.last, err = self.cpuTimes(false)
		return &CPUSample{}, nil
	}

	currentTimes, err := self.cpuTimes(false)
	if self.Debug() {
		helpers.LogStructureDetails(syslog, currentTimes, "CpuTimes", "raw", nil)
	}
	// in container envs we might get an empty array and the code panics after this
	if len(currentTimes) <= 0 {
		return &CPUSample{}, nil
	}

	delta := cpuDelta(&currentTimes[0], &self.last[0])
	self.last = currentTimes

	userDelta := delta.User + delta.Nice
	systemDelta := delta.System + delta.Irq + delta.Softirq
	stolenDelta := delta.Steal

	// Determine percentage values by dividing the total CPU time by each portion, then multiply by 100 to get a percentage from 0-100.
	var userPercent, stolenPercent, systemPercent, ioWaitPercent float64

	deltaTotal := delta.Total()
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
