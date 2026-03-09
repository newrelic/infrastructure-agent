// Copyright 2026 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux || darwin

package linux

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpstartService_SortKey(t *testing.T) {
	t.Run("standard service name", func(t *testing.T) {
		service := UpstartService{
			Name: "nginx",
			Pid:  "1234",
		}
		assert.Equal(t, "nginx", service.SortKey())
	})

	t.Run("service with special characters", func(t *testing.T) {
		service := UpstartService{
			Name: "network-manager",
			Pid:  "5678",
		}
		assert.Equal(t, "network-manager", service.SortKey())
	})

	t.Run("empty name", func(t *testing.T) {
		service := UpstartService{
			Name: "",
			Pid:  "0",
		}
		assert.Empty(t, service.SortKey())
	})
}

func TestUpstartService_Fields(t *testing.T) {
	service := UpstartService{
		Name: "test-service",
		Pid:  "12345",
	}

	assert.Equal(t, "test-service", service.Name)
	assert.Equal(t, "12345", service.Pid)
}

func TestUpstartPlugin_getUpstartDataset(t *testing.T) {
	t.Run("empty services", func(t *testing.T) {
		plugin := UpstartPlugin{ //nolint:exhaustruct
			runningServices: make(map[string]UpstartService),
		}
		dataset := plugin.getUpstartDataset()
		assert.Empty(t, dataset)
	})

	t.Run("single service", func(t *testing.T) {
		plugin := UpstartPlugin{ //nolint:exhaustruct
			runningServices: map[string]UpstartService{
				"nginx": {Name: "nginx", Pid: "1234"},
			},
		}
		dataset := plugin.getUpstartDataset()
		assert.Len(t, dataset, 1)
	})

	t.Run("multiple services", func(t *testing.T) {
		plugin := UpstartPlugin{ //nolint:exhaustruct
			runningServices: map[string]UpstartService{
				"nginx":   {Name: "nginx", Pid: "1234"},
				"mysql":   {Name: "mysql", Pid: "5678"},
				"postfix": {Name: "postfix", Pid: "9012"},
			},
		}
		dataset := plugin.getUpstartDataset()
		assert.Len(t, dataset, 3)
	})
}

func TestUpstartPlugin_getUpstartPidMap(t *testing.T) {
	t.Run("empty services", func(t *testing.T) {
		plugin := UpstartPlugin{ //nolint:exhaustruct
			runningServices: make(map[string]UpstartService),
		}
		pidMap := plugin.getUpstartPidMap()
		assert.Empty(t, pidMap)
	})

	t.Run("single service with valid pid", func(t *testing.T) {
		plugin := UpstartPlugin{ //nolint:exhaustruct
			runningServices: map[string]UpstartService{
				"nginx": {Name: "nginx", Pid: "1234"},
			},
		}
		pidMap := plugin.getUpstartPidMap()
		assert.Len(t, pidMap, 1)
		assert.Equal(t, "nginx", pidMap[1234])
	})

	t.Run("service with invalid pid", func(t *testing.T) {
		plugin := UpstartPlugin{ //nolint:exhaustruct
			runningServices: map[string]UpstartService{
				"nginx": {Name: "nginx", Pid: "unknown"},
			},
		}
		pidMap := plugin.getUpstartPidMap()
		assert.Empty(t, pidMap)
	})

	t.Run("multiple services", func(t *testing.T) {
		plugin := UpstartPlugin{ //nolint:exhaustruct
			runningServices: map[string]UpstartService{
				"nginx": {Name: "nginx", Pid: "1234"},
				"mysql": {Name: "mysql", Pid: "5678"},
			},
		}
		pidMap := plugin.getUpstartPidMap()
		assert.Len(t, pidMap, 2)
		assert.Equal(t, "nginx", pidMap[1234])
		assert.Equal(t, "mysql", pidMap[5678])
	})

	t.Run("mixed valid and invalid pids", func(t *testing.T) {
		plugin := UpstartPlugin{ //nolint:exhaustruct
			runningServices: map[string]UpstartService{
				"nginx":   {Name: "nginx", Pid: "1234"},
				"unknown": {Name: "unknown", Pid: "invalid"},
			},
		}
		pidMap := plugin.getUpstartPidMap()
		assert.Len(t, pidMap, 1)
		assert.Equal(t, "nginx", pidMap[1234])
	})
}
