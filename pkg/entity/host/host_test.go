// Copyright 2026 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package host

import (
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/sysinfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIDLookup_AgentKey(t *testing.T) {
	tests := []struct {
		name        string
		lookup      IDLookup
		expectedKey string
		expectError error
	}{
		{
			name:        "empty lookup returns error",
			lookup:      IDLookup{},
			expectedKey: "",
			expectError: ErrNoEntityKeys,
		},
		{
			name: "instance id takes priority",
			lookup: IDLookup{
				sysinfo.HOST_SOURCE_INSTANCE_ID: "i-12345",
				sysinfo.HOST_SOURCE_HOSTNAME:    "myhost",
			},
			expectedKey: "i-12345",
			expectError: nil,
		},
		{
			name: "hostname when no cloud id",
			lookup: IDLookup{
				sysinfo.HOST_SOURCE_HOSTNAME: "myhost.example.com",
			},
			expectedKey: "myhost.example.com",
			expectError: nil,
		},
		{
			name: "azure vm id",
			lookup: IDLookup{
				sysinfo.HOST_SOURCE_AZURE_VM_ID: "azure-vm-123",
				sysinfo.HOST_SOURCE_HOSTNAME:    "myhost",
			},
			expectedKey: "azure-vm-123",
			expectError: nil,
		},
		{
			name: "gcp vm id",
			lookup: IDLookup{
				sysinfo.HOST_SOURCE_GCP_VM_ID: "gcp-instance-456",
				sysinfo.HOST_SOURCE_HOSTNAME:  "myhost",
			},
			expectedKey: "gcp-instance-456",
			expectError: nil,
		},
		{
			name: "display name takes priority over hostname",
			lookup: IDLookup{
				sysinfo.HOST_SOURCE_DISPLAY_NAME: "My Display Name",
				sysinfo.HOST_SOURCE_HOSTNAME:     "myhost",
			},
			expectedKey: "My Display Name",
			expectError: nil,
		},
		{
			name: "skip empty values",
			lookup: IDLookup{
				sysinfo.HOST_SOURCE_INSTANCE_ID: "",
				sysinfo.HOST_SOURCE_HOSTNAME:    "myhost",
			},
			expectedKey: "myhost",
			expectError: nil,
		},
		{
			name: "undefined lookup type when only empty values",
			lookup: IDLookup{
				"unknown_key": "value",
			},
			expectedKey: "",
			expectError: ErrUndefinedLookupType,
		},
	}

	for _, tt := range tests { //nolint:varnamelen
		t.Run(tt.name, func(t *testing.T) {
			key, err := tt.lookup.AgentKey()
			if tt.expectError != nil {
				require.Error(t, err)
				assert.Equal(t, tt.expectError, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedKey, key)
			}
		})
	}
}

func TestIDLookup_AgentShortEntityName(t *testing.T) {
	tests := []struct {
		name         string
		lookup       IDLookup
		expectedName string
		expectError  error
	}{
		{
			name:         "empty lookup returns error",
			lookup:       IDLookup{},
			expectedName: "",
			expectError:  ErrUndefinedLookupType,
		},
		{
			name: "instance id takes priority",
			lookup: IDLookup{
				sysinfo.HOST_SOURCE_INSTANCE_ID:    "i-12345",
				sysinfo.HOST_SOURCE_HOSTNAME_SHORT: "myhost",
			},
			expectedName: "i-12345",
			expectError:  nil,
		},
		{
			name: "short hostname when no cloud id",
			lookup: IDLookup{
				sysinfo.HOST_SOURCE_HOSTNAME_SHORT: "myhost",
			},
			expectedName: "myhost",
			expectError:  nil,
		},
		{
			name: "display name takes priority over short hostname",
			lookup: IDLookup{
				sysinfo.HOST_SOURCE_DISPLAY_NAME:   "My Display Name",
				sysinfo.HOST_SOURCE_HOSTNAME_SHORT: "myhost",
			},
			expectedName: "My Display Name",
			expectError:  nil,
		},
		{
			name: "skip empty values",
			lookup: IDLookup{
				sysinfo.HOST_SOURCE_INSTANCE_ID:    "",
				sysinfo.HOST_SOURCE_HOSTNAME_SHORT: "myhost",
			},
			expectedName: "myhost",
			expectError:  nil,
		},
		{
			name: "alibaba vm id",
			lookup: IDLookup{
				sysinfo.HOST_SOURCE_ALIBABA_VM_ID:  "alibaba-123",
				sysinfo.HOST_SOURCE_HOSTNAME_SHORT: "myhost",
			},
			expectedName: "alibaba-123",
			expectError:  nil,
		},
		{
			name: "oci vm id",
			lookup: IDLookup{
				sysinfo.HOST_SOURCE_OCI_VM_ID:      "oci-456",
				sysinfo.HOST_SOURCE_HOSTNAME_SHORT: "myhost",
			},
			expectedName: "oci-456",
			expectError:  nil,
		},
	}

	for _, tt := range tests { //nolint:varnamelen
		t.Run(tt.name, func(t *testing.T) {
			name, err := tt.lookup.AgentShortEntityName()
			if tt.expectError != nil {
				require.Error(t, err)
				assert.Equal(t, tt.expectError, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedName, name)
			}
		})
	}
}

func TestErrorVariables(t *testing.T) {
	require.Error(t, ErrUndefinedLookupType)
	require.Error(t, ErrNoEntityKeys)

	assert.Equal(t, "no known identifier types found in ID lookup table", ErrUndefinedLookupType.Error())
	assert.Equal(t, "no agent identifiers available", ErrNoEntityKeys.Error())
}

func TestIDLookup_IsMap(t *testing.T) {
	lookup := IDLookup{
		"key1": "value1",
		"key2": "value2",
	}

	assert.Len(t, lookup, 2)
	assert.Equal(t, "value1", lookup["key1"])
	assert.Equal(t, "value2", lookup["key2"])
}
