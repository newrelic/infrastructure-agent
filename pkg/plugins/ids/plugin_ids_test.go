// Copyright 2026 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ids

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPluginID(t *testing.T) {
	tests := []struct {
		name     string
		category string
		term     string
	}{
		{
			name:     "standard plugin id",
			category: "metadata",
			term:     "system",
		},
		{
			name:     "empty category",
			category: "",
			term:     "system",
		},
		{
			name:     "empty term",
			category: "metadata",
			term:     "",
		},
		{
			name:     "empty both",
			category: "",
			term:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := NewPluginID(tt.category, tt.term)
			assert.Equal(t, tt.category, id.Category)
			assert.Equal(t, tt.term, id.Term)
		})
	}
}

func TestNewDefaultInventoryPluginID(t *testing.T) {
	tests := []struct {
		name string
		term string
	}{
		{
			name: "standard term",
			term: "myintegration",
		},
		{
			name: "empty term",
			term: "",
		},
		{
			name: "term with special characters",
			term: "my-integration_v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := NewDefaultInventoryPluginID(tt.term)
			assert.Equal(t, DefaultInventoryCategory, id.Category)
			assert.Equal(t, tt.term, id.Term)
		})
	}
}

func TestFromString(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		expected    PluginID
		expectError bool
	}{
		{
			name:        "valid source",
			source:      "metadata/system",
			expected:    PluginID{Category: "metadata", Term: "system"},
			expectError: false,
		},
		{
			name:        "valid source with special characters",
			source:      "my-category/my_term",
			expected:    PluginID{Category: "my-category", Term: "my_term"},
			expectError: false,
		},
		{
			name:        "invalid source no separator",
			source:      "metadatasystem",
			expected:    PluginID{Category: "", Term: ""},
			expectError: true,
		},
		{
			name:        "invalid source multiple separators",
			source:      "meta/data/system",
			expected:    PluginID{Category: "", Term: ""},
			expectError: true,
		},
		{
			name:        "empty source",
			source:      "",
			expected:    PluginID{Category: "", Term: ""},
			expectError: true,
		},
		{
			name:        "only separator",
			source:      "/",
			expected:    PluginID{Category: "", Term: ""},
			expectError: false,
		},
	}

	for _, tt := range tests { //nolint:varnamelen
		t.Run(tt.name, func(t *testing.T) {
			result, err := FromString(tt.source)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestPluginID_String(t *testing.T) {
	tests := []struct {
		name     string
		id       PluginID
		expected string
	}{
		{
			name:     "standard plugin id",
			id:       PluginID{Category: "metadata", Term: "system"},
			expected: "metadata/system",
		},
		{
			name:     "empty category",
			id:       PluginID{Category: "", Term: "system"},
			expected: "/system",
		},
		{
			name:     "empty term",
			id:       PluginID{Category: "metadata", Term: ""},
			expected: "metadata/",
		},
		{
			name:     "empty both",
			id:       PluginID{Category: "", Term: ""},
			expected: "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.id.String())
		})
	}
}

func TestPluginID_SortKey(t *testing.T) {
	tests := []struct {
		name string
		id   PluginID
	}{
		{
			name: "sort key equals string",
			id:   PluginID{Category: "metadata", Term: "system"},
		},
		{
			name: "empty plugin id",
			id:   PluginID{Category: "", Term: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.id.String(), tt.id.SortKey())
		})
	}
}

func TestPluginID_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		id       PluginID
		expected string
	}{
		{
			name:     "standard plugin id",
			id:       PluginID{Category: "metadata", Term: "system"},
			expected: `"metadata/system"`,
		},
		{
			name:     "empty plugin id",
			id:       PluginID{Category: "", Term: ""},
			expected: `"/"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.id)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(data))
		})
	}
}

func TestPluginID_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name        string
		jsonData    string
		expected    PluginID
		expectError bool
	}{
		{
			name:        "standard plugin id",
			jsonData:    `"metadata/system"`,
			expected:    PluginID{Category: "metadata", Term: "system"},
			expectError: false,
		},
		{
			name:        "empty category",
			jsonData:    `"/system"`,
			expected:    PluginID{Category: "", Term: "system"},
			expectError: false,
		},
		{
			name:        "invalid format",
			jsonData:    `"metadatasystem"`,
			expected:    PluginID{Category: "", Term: ""},
			expectError: true,
		},
	}

	for _, tt := range tests { //nolint:varnamelen
		t.Run(tt.name, func(t *testing.T) {
			var result PluginID

			err := json.Unmarshal([]byte(tt.jsonData), &result)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestPluginID_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		expected    PluginID
		expectError bool
	}{
		{
			name:        "standard plugin id",
			value:       "metadata/system",
			expected:    PluginID{Category: "metadata", Term: "system"},
			expectError: false,
		},
		{
			name:        "invalid format",
			value:       "metadatasystem",
			expected:    PluginID{Category: "", Term: ""},
			expectError: true,
		},
	}

	for _, tt := range tests { //nolint:varnamelen
		t.Run(tt.name, func(t *testing.T) {
			var result PluginID

			err := result.UnmarshalYAML(func(v any) error {
				strPtr, ok := v.(*string)
				if !ok {
					t.Fatal("expected *string type")
				}

				*strPtr = tt.value

				return nil
			})

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestPredefinedPluginIDs(t *testing.T) {
	assert.Equal(t, "metadata", CustomAttrsID.Category)
	assert.Equal(t, "attributes", CustomAttrsID.Term)

	assert.Equal(t, "metadata", HostInfo.Category)
	assert.Equal(t, "system", HostInfo.Term)

	assert.Empty(t, EmptyInventorySource.Category)
	assert.Empty(t, EmptyInventorySource.Term)
}

func TestDefaultInventoryCategory(t *testing.T) {
	assert.Equal(t, "integration", DefaultInventoryCategory)
}
