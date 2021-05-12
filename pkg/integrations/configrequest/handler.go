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
	NoopHandleFn = func(configProtocol protocol.ConfigProtocol) {}
)

type Entry struct {
	Definition integration.Definition
	YAMLConfig databind.YAMLConfig
}

type HandleFn func(cfgProtocol protocol.ConfigProtocol)

// NewHandleFn creates a handler func that runs every command within the request batch independently.
// Each command is run in parallel and won't depend on the results of the other ones.
func NewHandleFn(configProtocolQueue chan<- Entry, il integration.InstancesLookup, logger log.Entry) HandleFn {
	return func(cfgProtocol protocol.ConfigProtocol) {
		cfgRequest := cfgProtocol.BuildConfigRequest()
		for _, ce := range cfgProtocol.Integrations() {
			def, err := integration.NewDefinition(ce, il, nil, nil)
			if err != nil {
				logger.
					WithField("config_protocol_version", cfgProtocol.Version()).
					WithField("name", cfgRequest.ConfigName).
					WithError(err).
					Warn("cannot create handler for config protocol")
				return
			}
			if added := c.AddDefinition(cp.ConfigName, def); added {
				logger.
					WithField("config_name", cp.ConfigName).
					Debug("new definition added to the cache for the config name")
				def.ConfigRequest = cr
				//trace.CmdReq("queued definition: %+v", def)
				configProtocolQueue <- Entry{def.WithConfigRequest(cfgRequest), cfgProtocol.GetConfig()}
			}
		}
	}
}
