// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package host

import (
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo"
	"github.com/pkg/errors"
)

// Errors
var (
	ErrUndefinedLookupType = errors.New("no known identifier types found in ID lookup table")
	ErrNoEntityKeys        = errors.New("no agent identifiers available")
)

// IDLookup contains the identifiers used for resolving the agent entity name and agent key.
type IDLookup map[string]string

func (i IDLookup) AgentKey() (agentKey string, err error) {
	if len(i) == 0 {
		err = ErrNoEntityKeys
		return
	}

	for _, keyType := range sysinfo.HOST_ID_TYPES {
		// Skip blank identifiers which may have found their way into the map.
		// (Specifically, Azure can sometimes give us a blank VMID - See MTBLS-1429)
		if key, ok := i[keyType]; ok && key != "" {
			return key, nil
		}
	}

	err = ErrUndefinedLookupType
	return
}

// AgentShortEntityName is the agent entity name, but without having long-hostname into account.
// It is taken from the first field in the priority.
func (i IDLookup) AgentShortEntityName() (string, error) {
	priorities := []string{
		sysinfo.HOST_SOURCE_INSTANCE_ID,
		sysinfo.HOST_SOURCE_AZURE_VM_ID,
		sysinfo.HOST_SOURCE_GCP_VM_ID,
		sysinfo.HOST_SOURCE_ALIBABA_VM_ID,
		sysinfo.HOST_SOURCE_OCI_VM_ID,
		sysinfo.HOST_SOURCE_DISPLAY_NAME,
		sysinfo.HOST_SOURCE_HOSTNAME_SHORT,
	}

	for _, k := range priorities {
		if name, ok := i[k]; ok && name != "" {
			return name, nil
		}
	}

	return "", ErrUndefinedLookupType
}
