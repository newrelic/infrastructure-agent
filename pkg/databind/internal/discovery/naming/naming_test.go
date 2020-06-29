// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package naming

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAddPrefixToVariable(t *testing.T) {

	t.Parallel()
	tests := []struct {
		name     string
		prefix   string
		variable string
		expected string
	}{
		{
			name:     "simple",
			prefix:   "prefix",
			variable: "${test}",
			expected: "${prefix.test}",
		},
		{
			name:     "with_dot",
			prefix:   "prefix.",
			variable: "Prefix ends with a '.' ${test}",
			expected: "Prefix ends with a '.' ${prefix.test}",
		},
		{
			name:     "not_variable",
			prefix:   "prefix.",
			variable: "test",
			expected: "test",
		},
		{
			name:     "multiple",
			prefix:   "prefix.",
			variable: "something:${test}:something:${other}",
			expected: "something:${prefix.test}:something:${prefix.other}",
		},
		{
			name:     "ignore",
			prefix:   "prefix.",
			variable: "incomplete braces ${test:something so should be ignored",
			expected: "incomplete braces ${test:something so should be ignored",
		},
		{
			name:     "no_prefix",
			prefix:   "",
			variable: "Prefix blank so should not change ${test}",
			expected: "Prefix blank so should not change ${test}",
		},
		{
			name:     "ignore_prefix",
			prefix:   "prefix",
			variable: "String already has ${prefix.test} so should stay ${prefix.something}",
			expected: "String already has ${prefix.test} so should stay ${prefix.something}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, AddPrefixToVariable(tt.prefix, tt.variable))
		})
	}
}
