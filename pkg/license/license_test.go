// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package license

import (
	"testing"

	"gotest.tools/assert"
)

const (
	basic     = "0123456789012345678901234567890123456789"
	euLicense = "eu01xx6789012345678901234567890123456789"
	jpLicense = "jpxxxx6789012345678901234567890123456789"
)

func TestLicense_GetRegion(t *testing.T) {
	region := GetRegion(basic)
	assert.Equal(t, region, "")

	region = GetRegion(euLicense)
	assert.Equal(t, region, "eu")
}

func TestLicense_GetRegionPrefix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		key      string
		expected string
	}{
		{"basic key has no x", basic, ""},
		{"eu key", euLicense, "eu01"}, // eu still uses old logic, no change, doesn't flow through this func.
		{"jp key", jpLicense, "jp"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, GetRegionPrefix(tc.key))
		})
	}
}

func TestLicense_IsRegionJP(t *testing.T) {
	t.Parallel()

	assert.Equal(t, false, IsRegionJP(basic))
	assert.Equal(t, false, IsRegionJP(euLicense))
	assert.Equal(t, true, IsRegionJP(jpLicense))
}
