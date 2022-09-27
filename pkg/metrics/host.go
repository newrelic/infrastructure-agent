// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"fmt"

	"github.com/shirou/gopsutil/v3/host"
)

type HostSample struct {
	Uptime uint64 `json:"uptime"`
}

type HostMonitor struct{}

func NewHostMonitor() *HostMonitor {
	return &HostMonitor{}
}

func (m *HostMonitor) Sample() (*HostSample, error) {
	uptime, err := host.Uptime()
	if err != nil {
		return nil, fmt.Errorf("cannot sample uptime: %w", err)
	}

	return &HostSample{Uptime: uptime}, nil
}
