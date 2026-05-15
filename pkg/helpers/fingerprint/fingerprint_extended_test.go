// Copyright 2026 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package fingerprint

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFingerprint_Equals(t *testing.T) { //nolint:funlen
	tests := []struct {
		name     string
		fp1      Fingerprint
		fp2      Fingerprint
		expected bool
	}{
		{
			name: "identical fingerprints",
			fp1: Fingerprint{
				FullHostname:    "host.example.com",
				Hostname:        "host",
				CloudProviderId: "i-12345",
				DisplayName:     "My Host",
				BootID:          "boot-123",
				IpAddresses:     Addresses{"eth0": {"192.168.1.1"}},
				MacAddresses:    Addresses{"eth0": {"00:11:22:33:44:55"}},
			},
			fp2: Fingerprint{
				FullHostname:    "host.example.com",
				Hostname:        "host",
				CloudProviderId: "i-12345",
				DisplayName:     "My Host",
				BootID:          "boot-123",
				IpAddresses:     Addresses{"eth0": {"192.168.1.1"}},
				MacAddresses:    Addresses{"eth0": {"00:11:22:33:44:55"}},
			},
			expected: true,
		},
		{
			name: "different hostname",
			fp1: Fingerprint{
				FullHostname:    "",
				Hostname:        "host1",
				CloudProviderId: "",
				DisplayName:     "",
				BootID:          "",
				IpAddresses:     nil,
				MacAddresses:    nil,
			},
			fp2: Fingerprint{
				FullHostname:    "",
				Hostname:        "host2",
				CloudProviderId: "",
				DisplayName:     "",
				BootID:          "",
				IpAddresses:     nil,
				MacAddresses:    nil,
			},
			expected: false,
		},
		{
			name: "different full hostname",
			fp1: Fingerprint{
				FullHostname:    "host1.example.com",
				Hostname:        "",
				CloudProviderId: "",
				DisplayName:     "",
				BootID:          "",
				IpAddresses:     nil,
				MacAddresses:    nil,
			},
			fp2: Fingerprint{
				FullHostname:    "host2.example.com",
				Hostname:        "",
				CloudProviderId: "",
				DisplayName:     "",
				BootID:          "",
				IpAddresses:     nil,
				MacAddresses:    nil,
			},
			expected: false,
		},
		{
			name: "different cloud provider id",
			fp1: Fingerprint{
				FullHostname:    "",
				Hostname:        "",
				CloudProviderId: "i-12345",
				DisplayName:     "",
				BootID:          "",
				IpAddresses:     nil,
				MacAddresses:    nil,
			},
			fp2: Fingerprint{
				FullHostname:    "",
				Hostname:        "",
				CloudProviderId: "i-67890",
				DisplayName:     "",
				BootID:          "",
				IpAddresses:     nil,
				MacAddresses:    nil,
			},
			expected: false,
		},
		{
			name: "different boot id",
			fp1: Fingerprint{
				FullHostname:    "",
				Hostname:        "",
				CloudProviderId: "",
				DisplayName:     "",
				BootID:          "boot-123",
				IpAddresses:     nil,
				MacAddresses:    nil,
			},
			fp2: Fingerprint{
				FullHostname:    "",
				Hostname:        "",
				CloudProviderId: "",
				DisplayName:     "",
				BootID:          "boot-456",
				IpAddresses:     nil,
				MacAddresses:    nil,
			},
			expected: false,
		},
		{
			name: "different display name",
			fp1: Fingerprint{
				FullHostname:    "",
				Hostname:        "",
				CloudProviderId: "",
				DisplayName:     "Display 1",
				BootID:          "",
				IpAddresses:     nil,
				MacAddresses:    nil,
			},
			fp2: Fingerprint{
				FullHostname:    "",
				Hostname:        "",
				CloudProviderId: "",
				DisplayName:     "Display 2",
				BootID:          "",
				IpAddresses:     nil,
				MacAddresses:    nil,
			},
			expected: false,
		},
		{
			name: "different ip addresses",
			fp1: Fingerprint{
				FullHostname:    "",
				Hostname:        "",
				CloudProviderId: "",
				DisplayName:     "",
				BootID:          "",
				IpAddresses:     Addresses{"eth0": {"192.168.1.1"}},
				MacAddresses:    nil,
			},
			fp2: Fingerprint{
				FullHostname:    "",
				Hostname:        "",
				CloudProviderId: "",
				DisplayName:     "",
				BootID:          "",
				IpAddresses:     Addresses{"eth0": {"192.168.1.2"}},
				MacAddresses:    nil,
			},
			expected: false,
		},
		{
			name: "different mac addresses",
			fp1: Fingerprint{
				FullHostname:    "",
				Hostname:        "",
				CloudProviderId: "",
				DisplayName:     "",
				BootID:          "",
				IpAddresses:     nil,
				MacAddresses:    Addresses{"eth0": {"00:11:22:33:44:55"}},
			},
			fp2: Fingerprint{
				FullHostname:    "",
				Hostname:        "",
				CloudProviderId: "",
				DisplayName:     "",
				BootID:          "",
				IpAddresses:     nil,
				MacAddresses:    Addresses{"eth0": {"00:11:22:33:44:66"}},
			},
			expected: false,
		},
		{
			name: "empty fingerprints are equal",
			fp1: Fingerprint{
				FullHostname:    "",
				Hostname:        "",
				CloudProviderId: "",
				DisplayName:     "",
				BootID:          "",
				IpAddresses:     nil,
				MacAddresses:    nil,
			},
			fp2: Fingerprint{
				FullHostname:    "",
				Hostname:        "",
				CloudProviderId: "",
				DisplayName:     "",
				BootID:          "",
				IpAddresses:     nil,
				MacAddresses:    nil,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.fp1.Equals(tt.fp2)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAddresses_Equals(t *testing.T) {
	tests := []struct {
		name     string
		a        Addresses
		b        Addresses
		expected bool
	}{
		{
			name:     "both nil",
			a:        nil,
			b:        nil,
			expected: true,
		},
		{
			name:     "a nil b not nil",
			a:        nil,
			b:        Addresses{},
			expected: false,
		},
		{
			name:     "a not nil b nil",
			a:        Addresses{},
			b:        nil,
			expected: false,
		},
		{
			name:     "both empty",
			a:        Addresses{},
			b:        Addresses{},
			expected: true,
		},
		{
			name:     "same single entry",
			a:        Addresses{"eth0": {"192.168.1.1"}},
			b:        Addresses{"eth0": {"192.168.1.1"}},
			expected: true,
		},
		{
			name:     "different keys",
			a:        Addresses{"eth0": {"192.168.1.1"}},
			b:        Addresses{"eth1": {"192.168.1.1"}},
			expected: false,
		},
		{
			name:     "different values",
			a:        Addresses{"eth0": {"192.168.1.1"}},
			b:        Addresses{"eth0": {"192.168.1.2"}},
			expected: false,
		},
		{
			name:     "different number of keys",
			a:        Addresses{"eth0": {"192.168.1.1"}},
			b:        Addresses{"eth0": {"192.168.1.1"}, "eth1": {"192.168.1.2"}},
			expected: false,
		},
		{
			name:     "different number of values",
			a:        Addresses{"eth0": {"192.168.1.1"}},
			b:        Addresses{"eth0": {"192.168.1.1", "192.168.1.2"}},
			expected: false,
		},
		{
			name:     "same multiple entries",
			a:        Addresses{"eth0": {"192.168.1.1", "fe80::1"}, "lo": {"127.0.0.1"}},
			b:        Addresses{"eth0": {"192.168.1.1", "fe80::1"}, "lo": {"127.0.0.1"}},
			expected: true,
		},
		{
			name:     "same entries different order in slice",
			a:        Addresses{"eth0": {"192.168.1.1", "192.168.1.2"}},
			b:        Addresses{"eth0": {"192.168.1.2", "192.168.1.1"}},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.a.Equals(tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewHarvestor(t *testing.T) {
	tests := []struct {
		name        string
		config      any
		expectError bool
	}{
		{
			name:        "nil config returns error",
			config:      nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			harvester, err := NewHarvestor(nil, nil, nil)
			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, harvester)
			}
		})
	}
}

func TestMockHarvestor_Harvest(t *testing.T) {
	mock := &MockHarvestor{}
	fingerprint, err := mock.Harvest()

	require.NoError(t, err)
	assert.Equal(t, "test1.newrelic.com", fingerprint.FullHostname)
	assert.Equal(t, "test1", fingerprint.Hostname)
	assert.Equal(t, "1234abc", fingerprint.CloudProviderId)
	assert.Equal(t, "foobar", fingerprint.DisplayName)
	assert.Equal(t, "qwerty1234", fingerprint.BootID)
	assert.NotNil(t, fingerprint.IpAddresses)
	assert.NotNil(t, fingerprint.MacAddresses)
}
