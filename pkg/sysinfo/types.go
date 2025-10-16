// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package sysinfo

//
// Shared Types used in the Agent and subsystems
//

const (
	HOST_SOURCE_DISPLAY_NAME   = "display_name"
	HOST_SOURCE_INSTANCE_ID    = "instance-id"
	HOST_SOURCE_AZURE_VM_ID    = "azure_vm_id"
	HOST_SOURCE_GCP_VM_ID      = "gcp_vm_id"
	HOST_SOURCE_ALIBABA_VM_ID  = "alibaba_vm_id"
	HOST_SOURCE_OCI_VM_ID      = "oci_vm_id" //nolint:revive,stylecheck
	HOST_SOURCE_HOSTNAME       = "hostname"
	HOST_SOURCE_HOSTNAME_SHORT = "hostname_short"

	PROCESS_NAME_SOURCE_DAEMONTOOLS = "daemontools"
	PROCESS_NAME_SOURCE_SUPERVISOR  = "supervisor"
	PROCESS_NAME_SOURCE_SYSTEMD     = "systemd"
	PROCESS_NAME_SOURCE_SYSVINIT    = "sysvinit"
	PROCESS_NAME_SOURCE_UPSTART     = "upstart"
)

var (
	// Ordered list of which types of names to prefer for coming up with the agent identifier.
	// The first one in the list which we have will win.
	HOST_ID_TYPES = []string{
		HOST_SOURCE_INSTANCE_ID,
		HOST_SOURCE_AZURE_VM_ID,
		HOST_SOURCE_GCP_VM_ID,
		HOST_SOURCE_ALIBABA_VM_ID,
		HOST_SOURCE_OCI_VM_ID,
		HOST_SOURCE_DISPLAY_NAME,
		HOST_SOURCE_HOSTNAME,
	}

	// Ordered list of which types of process name sources we can use. The first one in the
	// list which has a match for a PID will win.
	PROCESS_NAME_SOURCES = []string{
		// There's no clear ordering among this group - they're all top-level service managers.
		PROCESS_NAME_SOURCE_DAEMONTOOLS,
		PROCESS_NAME_SOURCE_SUPERVISOR,
		PROCESS_NAME_SOURCE_SYSTEMD,
		PROCESS_NAME_SOURCE_UPSTART,

		// AKA pidfiles. This goes last, as it's common to have a pidfile for something which is defined in a real service manager
		PROCESS_NAME_SOURCE_SYSVINIT,
	}
)

type HostAliases struct {
	Alias  string `json:"alias"`
	Source string `json:"id"`
}

func (self HostAliases) SortKey() string {
	return self.Source
}
