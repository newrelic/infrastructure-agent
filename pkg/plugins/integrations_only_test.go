// Copyright 2026 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package plugins

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIntegrationsOnly_SortKey(t *testing.T) {
	tests := []struct {
		name     string
		io       IntegrationsOnly
		expected string
	}{
		{
			name:     "true value",
			io:       IntegrationsOnly(true),
			expected: "integrations_only",
		},
		{
			name:     "false value",
			io:       IntegrationsOnly(false),
			expected: "integrations_only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.io.SortKey())
		})
	}
}

func TestIntegrationsOnly_BoolValue(t *testing.T) {
	trueValue := IntegrationsOnly(true)
	falseValue := IntegrationsOnly(false)

	assert.True(t, bool(trueValue))
	assert.False(t, bool(falseValue))
}
