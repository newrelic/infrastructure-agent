package configrequest

import (
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/cache"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/databind"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/configrequest/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

var (
	// helper for testing purposes
	NoopHandleFn = func(configProtocol protocol.ConfigProtocol, c cache.Cache, parentDefinition integration.Definition) {}
)

type Entry struct {
	Definition integration.Definition
	Databind   databind.YAMLConfig
}

type HandleFn func(cfgProtocol protocol.ConfigProtocol, c cache.Cache, parentDefinition integration.Definition)

// NewHandleFn creates a handler func that runs every command within the request batch independently.
// Each command is run in parallel and won't depend on the results of the other ones.
func NewHandleFn(configProtocolQueue chan<- Entry, terminateDefinitionQueue chan<- string, il integration.InstancesLookup, logger log.Entry) HandleFn {
	return func(cfgProtocol protocol.ConfigProtocol, c cache.Cache, parentDefinition integration.Definition) {
		cfgDefinitions := c.TakeConfig(cfgProtocol.Name())
		for _, ce := range cfgProtocol.Integrations() {
			def, err := integration.NewDefinition(ce, il, nil, nil)
			if err != nil {
				logger.
					WithField("config_protocol_version", cfgProtocol.Version()).
					WithField("name", cfgProtocol.Name()).
					WithError(err).
					Warn("cannot create handler for config protocol")
				return
			}
			def.CfgProtocol = protocol.Context{ParentName: parentDefinition.Name, ConfigName: cfgProtocol.Name()}
			if cfgDefinitions.Add(def) {
				logger.
					WithField("config_name", cfgProtocol.Name()).
					Debug("new definition added to the cache for the config name")
				configProtocolQueue <- Entry{def, cfgProtocol.GetConfig()}
			}
		}
		removedDefinitions := c.ApplyConfig(cfgDefinitions)
		for _, hash := range removedDefinitions {
			terminateDefinitionQueue <- hash
		}
	}
}
