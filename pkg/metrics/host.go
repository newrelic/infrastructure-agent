// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v3/host"
)

type HostSample struct {
	Uptime    uint64   `json:"uptime"`
	NtpOffset *float64 `json:"ntpOffset,omitempty"`
}

type HostMonitor struct {
	ntpMonitor NtpMonitor
}

type NtpMonitor interface {
	Offset() (time.Duration, error)
	ValidInterval() bool
}

func NewHostMonitor(ntpMonitor NtpMonitor) *HostMonitor {
	return &HostMonitor{ntpMonitor: ntpMonitor}
}

func (m *HostMonitor) Sample() (*HostSample, error) {
	hostSample := &HostSample{}
	uptime, err := host.Uptime()
	if err != nil {
		return nil, fmt.Errorf("cannot sample uptime: %w", err)
	}
	hostSample.Uptime = uptime

	if m.ntpMonitor != nil {
		if m.ntpMonitor.ValidInterval() {
			ntpOffset, err := m.ntpMonitor.Offset()
			if err != nil {
				syslog.WithError(err).Error("cannot get ntp offset")
			} else {
				seconds := ntpOffset.Seconds()
				hostSample.NtpOffset = &seconds
			}
		}
	}

	return hostSample, nil
}
