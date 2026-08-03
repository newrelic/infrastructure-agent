// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterExcludedTags(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		tags            map[string]string
		excludePatterns []string
		expected        map[string]string
	}{
		{
			name:            "no exclude patterns",
			tags:            map[string]string{"env": "prod"},
			excludePatterns: nil,
			expected:        map[string]string{"env": "prod"},
		},
		{
			name:            "no tags",
			tags:            nil,
			excludePatterns: []string{"pipeline-*"},
			expected:        nil,
		},
		{
			name:            "exact match excluded",
			tags:            map[string]string{"env": "prod", "team": "infra"},
			excludePatterns: []string{"team"},
			expected:        map[string]string{"env": "prod"},
		},
		{
			name:            "glob match excluded",
			tags:            map[string]string{"env": "prod", "pipeline-run-id": "123", "pipeline-stage": "build"},
			excludePatterns: []string{"pipeline-*"},
			expected:        map[string]string{"env": "prod"},
		},
		{
			name:            "no match leaves tags untouched",
			tags:            map[string]string{"env": "prod"},
			excludePatterns: []string{"pipeline-*"},
			expected:        map[string]string{"env": "prod"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.expected, filterExcludedTags(tc.tags, tc.excludePatterns))
		})
	}
}
