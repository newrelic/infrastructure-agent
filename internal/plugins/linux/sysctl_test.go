// Copyright 2026 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux || darwin

package linux

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSysctlItem_SortKey(t *testing.T) {
	tests := []struct {
		name     string
		item     SysctlItem
		expected string
	}{
		{
			name: "standard sysctl",
			item: SysctlItem{
				Sysctl: "net.ipv4.tcp_syncookies",
				Value:  "1",
			},
			expected: "net.ipv4.tcp_syncookies",
		},
		{
			name: "kernel sysctl",
			item: SysctlItem{
				Sysctl: "kernel.panic",
				Value:  "0",
			},
			expected: "kernel.panic",
		},
		{
			name: "empty sysctl",
			item: SysctlItem{
				Sysctl: "",
				Value:  "",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.item.SortKey())
		})
	}
}

func TestSysctlItem_Fields(t *testing.T) {
	item := SysctlItem{
		Sysctl: "vm.swappiness",
		Value:  "60",
	}

	assert.Equal(t, "vm.swappiness", item.Sysctl)
	assert.Equal(t, "60", item.Value)
}

func TestSysctlConstants(t *testing.T) {
	assert.Equal(t, 0o222, WRITABLE_MASK)
	assert.Equal(t, 0o444, READABLE_MASK)
}

func TestIgnoredListPatterns(t *testing.T) {
	// Verify that ignoredListPatterns is defined and has expected patterns
	assert.NotEmpty(t, ignoredListPatterns)
	assert.Greater(t, len(ignoredListPatterns), 5)

	// Check some specific patterns are present
	foundKernelPattern := false
	foundNetPattern := false
	foundFsPattern := false

	for _, pattern := range ignoredListPatterns {
		switch {
		case len(pattern) >= 6 && pattern[:6] == "kernel":
			foundKernelPattern = true
		case len(pattern) >= 3 && pattern[:3] == "net":
			foundNetPattern = true
		case len(pattern) >= 2 && pattern[:2] == "fs":
			foundFsPattern = true
		}
	}

	assert.True(t, foundKernelPattern, "Expected kernel pattern in ignoredListPatterns")
	assert.True(t, foundNetPattern, "Expected net pattern in ignoredListPatterns")
	assert.True(t, foundFsPattern, "Expected fs pattern in ignoredListPatterns")
}
