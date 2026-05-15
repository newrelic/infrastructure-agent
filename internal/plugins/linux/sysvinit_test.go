// Copyright 2026 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux

package linux

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSysvService_SortKey(t *testing.T) {
	tests := []struct {
		name     string
		service  SysvService
		expected string
	}{
		{
			name: "standard service name",
			service: SysvService{
				Name: "nginx",
			},
			expected: "nginx",
		},
		{
			name: "service with hyphen",
			service: SysvService{
				Name: "network-manager",
			},
			expected: "network-manager",
		},
		{
			name: "empty name",
			service: SysvService{
				Name: "",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.service.SortKey())
		})
	}
}

func TestSysvService_Fields(t *testing.T) {
	service := SysvService{
		Name: "test-service",
	}

	assert.Equal(t, "test-service", service.Name)
}

func TestSysvInitDir(t *testing.T) {
	assert.Equal(t, "/var/run", SYSV_INIT_DIR)
}

func TestPidFileIsStale(t *testing.T) {
	tests := []struct {
		name             string
		pidFileMod       time.Time
		processStartTime time.Time
		expected         bool
	}{
		{
			name:             "not stale - process started before pidfile",
			pidFileMod:       time.Now(),
			processStartTime: time.Now().Add(-1 * time.Second),
			expected:         false,
		},
		{
			name:             "not stale - same time",
			pidFileMod:       time.Now(),
			processStartTime: time.Now(),
			expected:         false,
		},
		{
			name:             "not stale - within 5 second threshold",
			pidFileMod:       time.Now().Add(-3 * time.Second),
			processStartTime: time.Now(),
			expected:         false,
		},
		{
			name:             "stale - process started more than 5 seconds after pidfile",
			pidFileMod:       time.Now().Add(-10 * time.Second),
			processStartTime: time.Now(),
			expected:         true,
		},
		{
			name:             "stale - large time difference",
			pidFileMod:       time.Now().Add(-1 * time.Hour),
			processStartTime: time.Now(),
			expected:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pidFileIsStale(tt.pidFileMod, tt.processStartTime)
			assert.Equal(t, tt.expected, result)
		})
	}
}
