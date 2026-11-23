//go:build linux || darwin
// +build linux darwin

// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/shirou/gopsutil/v3/cpu"
)

// NewCPUMonitor creates a new CPU monitor using gopsutil for Unix-like platforms (Linux and macOS).
func NewCPUMonitor(context agent.AgentContext) *CPUMonitor {
	return &CPUMonitor{
		context:        context,
		last:           nil,
		cpuTimes:       cpu.Times,
		windowsMonitor: nil,
	}
}
