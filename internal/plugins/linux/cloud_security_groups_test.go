// Copyright 2026 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux || darwin

package linux

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCloudSecurityGroup_SortKey(t *testing.T) {
	tests := []struct {
		name     string
		group    CloudSecurityGroup
		expected string
	}{
		{
			name: "standard security group",
			group: CloudSecurityGroup{
				SecurityGroup: "sg-12345678",
			},
			expected: "sg-12345678",
		},
		{
			name: "security group with name",
			group: CloudSecurityGroup{
				SecurityGroup: "default",
			},
			expected: "default",
		},
		{
			name: "empty security group",
			group: CloudSecurityGroup{
				SecurityGroup: "",
			},
			expected: "",
		},
		{
			name: "security group with special characters",
			group: CloudSecurityGroup{
				SecurityGroup: "my-security-group_v1",
			},
			expected: "my-security-group_v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.group.SortKey())
		})
	}
}

func TestCloudSecurityGroup_Fields(t *testing.T) {
	group := CloudSecurityGroup{
		SecurityGroup: "sg-abcdef123",
	}

	assert.Equal(t, "sg-abcdef123", group.SecurityGroup)
}
