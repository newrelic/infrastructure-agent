//go:build !windows
// +build !windows

// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metrics

import (
	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/shirou/gopsutil/v3/cpu"
)

// NewCPUMonitor creates a new CPU monitor using gopsutil for non-Windows platforms.
func NewCPUMonitor(context agent.AgentContext) *CPUMonitor {
	return &CPUMonitor{
		context:        context,
		cpuTimes:       cpu.Times,
		last:           nil,
		windowsMonitor: nil,
	}
}
