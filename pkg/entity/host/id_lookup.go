package host

import (
	"fmt"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/hostname"
)

// IDLookup contains the identifiers used for resolving the agent entity name and agent key.
type IDLookup map[string]string

func (i IDLookup) getAgentKey() (agentKey string, err error) {
	if len(i) == 0 {
		err = fmt.Errorf("No identifiers given")
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

// NewIdLookup creates a new agent ID lookup table.
func NewIdLookup(resolver hostname.Resolver, cloudHarvester cloud.Harvester, displayName string) IDLookup {
	idLookupTable := make(IDLookup)
	// Attempt to get the hostname
	host, short, err := resolver.Query()
	llog := agent.alog.WithField("displayName", displayName)
	if err == nil {
		idLookupTable[sysinfo.HOST_SOURCE_HOSTNAME] = host
		idLookupTable[sysinfo.HOST_SOURCE_HOSTNAME_SHORT] = short
	} else {
		llog.WithError(err).Warn("could not determine hostname")
	}
	if host == "localhost" {
		llog.Warn("Localhost is not a good identifier")
	}
	// See if we have a configured alias which is not equal to the hostname, if so, use
	// it as a unique identifier and ignore the hostname
	if displayName != "" {
		idLookupTable[sysinfo.HOST_SOURCE_DISPLAY_NAME] = displayName
	}
	cloudInstanceID, err := cloudHarvester.GetInstanceID()
	if err != nil {
		llog.WithField("idLookupTable", idLookupTable).WithError(err).Debug("Unable to get instance id.")
	} else {
		idLookupTable[sysinfo.HOST_SOURCE_INSTANCE_ID] = cloudInstanceID
	}

	return idLookupTable
}

