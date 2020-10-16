package host

import (
	"strings"

	"github.com/newrelic/infrastructure-agent/pkg/backend/http"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
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

func ResolveUniqueEntityKey(e entity.Fields, agentID string, lookup IDLookup, entityRewrite []data.EntityRewrite, protocol int) (entity.Key, error) {
	if e.IsAgent() {
		return entity.Key(agentID), nil
	}

	name := ApplyEntityRewrite(e.Name, entityRewrite)

	result, err := ReplaceLoopback(name, lookup, protocol)
	if err != nil {
		return entity.EmptyKey, err
	}

	e.Name = result
	return e.Key()
}

func ReplaceLoopback(value string, lookup IDLookup, protocolVersion int) (string, error) {
	if protocolVersion < protocol.V3 || !http.ContainsLocalhost(value) {
		return value, nil
	}

	agentShortName, err := lookup.AgentShortEntityName()
	if err != nil {
		return "", err
	}

	return http.ReplaceLocalhost(value, agentShortName), nil
}

const (
	entityRewriteActionReplace = "replace"
)

// Try to match and replace entityName according to EntityRewrite configuration.
func ApplyEntityRewrite(entityName string, entityRewrite []data.EntityRewrite) string {
	result := entityName

	for _, er := range entityRewrite {
		if er.Action == entityRewriteActionReplace {
			result = strings.Replace(result, er.Match, er.ReplaceField, -1)
		}
	}

	return result
}
