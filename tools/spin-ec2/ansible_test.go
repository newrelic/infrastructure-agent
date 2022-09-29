// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestProvisionOptionsToString(t *testing.T) {

	tests := []struct {
		name        string
		filter      []int
		expected    string
		expectedErr error
	}{
		{
			name:        "empty",
			filter:      []int{},
			expected:    "",
			expectedErr: nil,
		},
		{
			name:        "one option",
			filter:      []int{0},
			expected:    "nothing",
			expectedErr: nil,
		},
		{
			name:        "multiple options",
			filter:      []int{0, 3},
			expected:    "nothing\n - package tests from PROD",
			expectedErr: nil,
		},
	}
	opts := newProvisionOptions()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filteredOpts, err := opts.filter(tt.filter)
			assert.Equal(t, tt.expectedErr, err)
			assert.Equal(t, tt.expected, filteredOpts.toString())
		})
	}

}
