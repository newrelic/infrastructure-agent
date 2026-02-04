// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package license

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	basic = "0123456789012345678901234567890123456789"
	eu    = "eu01xx6789012345678901234567890123456789"
	gov   = "gov123456789012345678901234567890123456789"
)

func TestLicense_GetRegion(t *testing.T) {
	tests := []struct {
		name     string
		license  string
		expected string
	}{
		{
			name:     "basic license has no region",
			license:  basic,
			expected: "",
		},
		{
			name:     "eu license",
			license:  eu,
			expected: "eu",
		},
		{
			name:     "gov license",
			license:  gov,
			expected: "gov",
		},
		{
			name:     "empty license",
			license:  "",
			expected: "",
		},
		{
			name:     "short license",
			license:  "ab",
			expected: "ab",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			region := GetRegion(tt.license)
			assert.Equal(t, tt.expected, region)
		})
	}
}

func TestLicense_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		license  string
		expected bool
	}{
		{
			name:     "valid alphanumeric license",
			license:  basic,
			expected: true,
		},
		{
			name:     "valid eu license",
			license:  eu,
			expected: true,
		},
		{
			name:     "valid gov license",
			license:  gov,
			expected: true,
		},
		{
			name:     "empty license is invalid",
			license:  "",
			expected: false,
		},
		{
			name:     "license with special characters is invalid",
			license:  "abc-123-def",
			expected: false,
		},
		{
			name:     "license with spaces is invalid",
			license:  "abc 123 def",
			expected: false,
		},
		{
			name:     "license with underscores is invalid",
			license:  "abc_123_def",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValid(tt.license)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLicense_IsRegionEU(t *testing.T) {
	tests := []struct {
		name     string
		license  string
		expected bool
	}{
		{
			name:     "basic license is not EU",
			license:  basic,
			expected: false,
		},
		{
			name:     "eu license is EU",
			license:  eu,
			expected: true,
		},
		{
			name:     "gov license is not EU",
			license:  gov,
			expected: false,
		},
		{
			name:     "empty license is not EU",
			license:  "",
			expected: false,
		},
		{
			name:     "eu01 prefix is EU",
			license:  "eu01abcdefghijklmnop",
			expected: true,
		},
		{
			name:     "single char is not EU",
			license:  "e",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRegionEU(tt.license)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLicense_IsFederalCompliance(t *testing.T) {
	tests := []struct {
		name     string
		license  string
		expected bool
	}{
		{
			name:     "basic license is not federal",
			license:  basic,
			expected: false,
		},
		{
			name:     "eu license is not federal",
			license:  eu,
			expected: false,
		},
		{
			name:     "gov license is federal",
			license:  gov,
			expected: true,
		},
		{
			name:     "empty license is not federal",
			license:  "",
			expected: false,
		},
		{
			name:     "license starting with gov is federal",
			license:  "gov0123456789",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsFederalCompliance(tt.license)
			assert.Equal(t, tt.expected, result)
		})
	}
}
