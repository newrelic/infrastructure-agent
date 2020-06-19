// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package fixture

import (
	"github.com/newrelic/infrastructure-agent/pkg/metrics"
	"github.com/newrelic/infrastructure-agent/pkg/sample"
)

var SystemSample = metrics.SystemSample{
	BaseEvent: sample.BaseEvent{
		EntityKey: "my-entity-key",
	},
	CPUSample: &metrics.CPUSample{
		CPUPercent:       1,
		CPUUserPercent:   2,
		CPUSystemPercent: 3,
		CPUIOWaitPercent: 4,
		CPUIdlePercent:   5,
		CPUStealPercent:  6,
	},
	LoadSample: &metrics.LoadSample{
		LoadOne:     1,
		LoadFive:    5,
		LoadFifteen: 15,
	},
	MemorySample: &metrics.MemorySample{
		MemoryTotal: 1,
		MemoryFree:  2,
		MemoryUsed:  3,
		SwapTotal:   4,
		SwapFree:    5,
		SwapUsed:    6,
	},
	DiskSample: &metrics.DiskSample{
		UsedBytes:               1,
		UsedPercent:             2,
		FreeBytes:               3,
		FreePercent:             4,
		TotalBytes:              5,
		UtilizationPercent:      6,
		ReadUtilizationPercent:  7,
		WriteUtilizationPercent: 8,
		ReadsPerSec:             9,
		WritesPerSec:            10,
	},
}
