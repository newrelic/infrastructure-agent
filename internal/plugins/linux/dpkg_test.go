// Copyright 2026 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux

package linux

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDpkgItem_SortKey(t *testing.T) {
	tests := []struct {
		name     string
		item     DpkgItem
		expected string
	}{
		{
			name: "standard package name",
			item: DpkgItem{
				Name:         "nginx",
				Architecture: "amd64",
				Version:      "1.18.0",
			},
			expected: "nginx",
		},
		{
			name: "package with special characters",
			item: DpkgItem{
				Name: "libc6-dev",
			},
			expected: "libc6-dev",
		},
		{
			name: "empty name",
			item: DpkgItem{
				Name: "",
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

func TestDpkgItem_Fields(t *testing.T) {
	item := DpkgItem{
		Name:         "test-package",
		Architecture: "amd64",
		Essential:    "no",
		Priority:     "optional",
		Status:       "installed",
		Version:      "1.0.0-1ubuntu1",
		InstallTime:  "1609459200",
	}

	assert.Equal(t, "test-package", item.Name)
	assert.Equal(t, "amd64", item.Architecture)
	assert.Equal(t, "no", item.Essential)
	assert.Equal(t, "optional", item.Priority)
	assert.Equal(t, "installed", item.Status)
	assert.Equal(t, "1.0.0-1ubuntu1", item.Version)
	assert.Equal(t, "1609459200", item.InstallTime)
}

func TestDpkgConstants(t *testing.T) {
	assert.Equal(t, "/var/lib/dpkg/info", DPKG_INFO_DIR)
	assert.Equal(t, 16, FSN_CLOSE_WRITE)
}
