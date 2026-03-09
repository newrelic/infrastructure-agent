// Copyright 2026 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package sysinfo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHostAliases_SortKey(t *testing.T) {
	tests := []struct {
		name     string
		alias    HostAliases
		expected string
	}{
		{
			name: "hostname source",
			alias: HostAliases{
				Alias:  "my-host.example.com",
				Source: HOST_SOURCE_HOSTNAME,
			},
			expected: HOST_SOURCE_HOSTNAME,
		},
		{
			name: "display name source",
			alias: HostAliases{
				Alias:  "My Display Name",
				Source: HOST_SOURCE_DISPLAY_NAME,
			},
			expected: HOST_SOURCE_DISPLAY_NAME,
		},
		{
			name: "instance id source",
			alias: HostAliases{
				Alias:  "i-1234567890abcdef0",
				Source: HOST_SOURCE_INSTANCE_ID,
			},
			expected: HOST_SOURCE_INSTANCE_ID,
		},
		{
			name: "empty source",
			alias: HostAliases{
				Alias:  "some-alias",
				Source: "",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.alias.SortKey())
		})
	}
}

func TestHostSourceConstants(t *testing.T) {
	assert.Equal(t, "display_name", HOST_SOURCE_DISPLAY_NAME)
	assert.Equal(t, "instance-id", HOST_SOURCE_INSTANCE_ID)
	assert.Equal(t, "azure_vm_id", HOST_SOURCE_AZURE_VM_ID)
	assert.Equal(t, "gcp_vm_id", HOST_SOURCE_GCP_VM_ID)
	assert.Equal(t, "alibaba_vm_id", HOST_SOURCE_ALIBABA_VM_ID)
	assert.Equal(t, "oci_vm_id", HOST_SOURCE_OCI_VM_ID)
	assert.Equal(t, "hostname", HOST_SOURCE_HOSTNAME)
	assert.Equal(t, "hostname_short", HOST_SOURCE_HOSTNAME_SHORT)
}

func TestProcessNameSourceConstants(t *testing.T) {
	assert.Equal(t, "daemontools", PROCESS_NAME_SOURCE_DAEMONTOOLS)
	assert.Equal(t, "supervisor", PROCESS_NAME_SOURCE_SUPERVISOR)
	assert.Equal(t, "systemd", PROCESS_NAME_SOURCE_SYSTEMD)
	assert.Equal(t, "sysvinit", PROCESS_NAME_SOURCE_SYSVINIT)
	assert.Equal(t, "upstart", PROCESS_NAME_SOURCE_UPSTART)
}

func TestHostIDTypes(t *testing.T) {
	expected := []string{
		HOST_SOURCE_INSTANCE_ID,
		HOST_SOURCE_AZURE_VM_ID,
		HOST_SOURCE_GCP_VM_ID,
		HOST_SOURCE_ALIBABA_VM_ID,
		HOST_SOURCE_OCI_VM_ID,
		HOST_SOURCE_DISPLAY_NAME,
		HOST_SOURCE_HOSTNAME,
	}
	assert.Equal(t, expected, HOST_ID_TYPES)
}

func TestProcessNameSources(t *testing.T) {
	expected := []string{
		PROCESS_NAME_SOURCE_DAEMONTOOLS,
		PROCESS_NAME_SOURCE_SUPERVISOR,
		PROCESS_NAME_SOURCE_SYSTEMD,
		PROCESS_NAME_SOURCE_UPSTART,
		PROCESS_NAME_SOURCE_SYSVINIT,
	}
	assert.Equal(t, expected, PROCESS_NAME_SOURCES)
}

func TestHostAliases_Fields(t *testing.T) {
	alias := HostAliases{
		Alias:  "test-alias",
		Source: "test-source",
	}

	assert.Equal(t, "test-alias", alias.Alias)
	assert.Equal(t, "test-source", alias.Source)
}
