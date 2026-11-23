// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"errors"
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
	ntpOffset  *float64 // cache for last offset value retrieved
}

type NtpMonitor interface {
	Offset() (time.Duration, error)
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
		ntpOffset, err := m.ntpMonitor.Offset()
		if err != nil {
			// skip the error and use cached offset if interval error
			if !errors.Is(err, ErrNotInInterval) {
				syslog.WithError(err).Error("cannot get ntp offset")
				m.ntpOffset = nil
			}
		} else {
			seconds := ntpOffset.Seconds()
			m.ntpOffset = &seconds
		}
		hostSample.NtpOffset = m.ntpOffset
	}

	return hostSample, nil
}
