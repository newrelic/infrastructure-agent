// Copyright 2026 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux || darwin

package linux

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKernelModule_SortKey(t *testing.T) {
	t.Run("standard module name", func(t *testing.T) {
		module := KernelModule{
			Name:        "ext4",
			Version:     "1.0",
			Description: "Fourth Extended Filesystem",
		}
		assert.Equal(t, "ext4", module.SortKey())
	})

	t.Run("empty name", func(t *testing.T) {
		module := KernelModule{
			Name:        "",
			Version:     "",
			Description: "",
		}
		assert.Empty(t, module.SortKey())
	})

	t.Run("module with special characters", func(t *testing.T) {
		module := KernelModule{
			Name:        "nf_conntrack_ipv4",
			Version:     "",
			Description: "",
		}
		assert.Equal(t, "nf_conntrack_ipv4", module.SortKey())
	})
}

func TestKernelModule_Fields(t *testing.T) {
	module := KernelModule{
		Name:        "test_module",
		Version:     "2.0.1",
		Description: "Test module description",
	}

	assert.Equal(t, "test_module", module.Name)
	assert.Equal(t, "2.0.1", module.Version)
	assert.Equal(t, "Test module description", module.Description)
}

func TestKernelModulesPlugin_getKernelModulesDataset(t *testing.T) {
	t.Run("empty modules", func(t *testing.T) {
		plugin := &KernelModulesPlugin{ //nolint:exhaustruct
			loadedModules: make(map[string]KernelModule),
		}
		dataset := plugin.getKernelModulesDataset()
		assert.Empty(t, dataset)
	})

	t.Run("single module", func(t *testing.T) {
		plugin := &KernelModulesPlugin{ //nolint:exhaustruct
			loadedModules: map[string]KernelModule{
				"ext4": {Name: "ext4", Version: "1.0", Description: ""},
			},
		}
		dataset := plugin.getKernelModulesDataset()
		assert.Len(t, dataset, 1)
	})

	t.Run("multiple modules", func(t *testing.T) {
		plugin := &KernelModulesPlugin{ //nolint:exhaustruct
			loadedModules: map[string]KernelModule{
				"ext4":  {Name: "ext4", Version: "1.0", Description: ""},
				"xfs":   {Name: "xfs", Version: "1.0", Description: ""},
				"btrfs": {Name: "btrfs", Version: "1.0", Description: ""},
			},
		}
		dataset := plugin.getKernelModulesDataset()
		assert.Len(t, dataset, 3)
	})
}

func TestKernelModulesPlugin_processUpdates_Additions(t *testing.T) {
	plugin := &KernelModulesPlugin{ //nolint:exhaustruct
		loadedModules: make(map[string]KernelModule),
		needsFlush:    false,
	}

	seenModules := map[string]bool{
		"newmodule": true,
	}

	// Note: This will fail to get module info since modinfo is not available in test
	// but it will still add the module to the map
	_ = plugin.processUpdates(seenModules)

	// Module should be added to the map even if modinfo fails
	assert.Contains(t, plugin.loadedModules, "newmodule")
}

func TestKernelModulesPlugin_processUpdates_Removals(t *testing.T) {
	plugin := &KernelModulesPlugin{ //nolint:exhaustruct
		loadedModules: map[string]KernelModule{
			"oldmodule": {Name: "oldmodule", Version: "", Description: ""},
		},
		needsFlush: false,
	}

	seenModules := map[string]bool{} // empty - module was removed

	err := plugin.processUpdates(seenModules)
	require.NoError(t, err)

	assert.NotContains(t, plugin.loadedModules, "oldmodule")
	assert.True(t, plugin.needsFlush)
}

func TestKernelModulesPlugin_processUpdates_NoChanges(t *testing.T) {
	plugin := &KernelModulesPlugin{ //nolint:exhaustruct
		loadedModules: map[string]KernelModule{
			"existingmodule": {Name: "existingmodule", Version: "", Description: ""},
		},
		needsFlush: false,
	}

	seenModules := map[string]bool{
		"existingmodule": true,
	}

	err := plugin.processUpdates(seenModules)
	require.NoError(t, err)

	assert.Contains(t, plugin.loadedModules, "existingmodule")
	assert.False(t, plugin.needsFlush) // No changes, should remain false
}
