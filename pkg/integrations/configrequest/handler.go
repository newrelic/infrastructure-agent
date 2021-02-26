package configrequest

import (
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/databind"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/configrequest/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/track/ctx"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

var (
	// helper for testing purposes
	NoopHandleFn = func(protocol.ConfigProtocolV1) {}
)

type Entry struct {
	Definition integration.Definition
	YAMLConfig databind.YAMLConfig
}

type HandleFn func(protocol.ConfigProtocolV1)

// NewHandleFn creates a handler func that runs every command within the request batch independently.
// Each command is run in parallel and won't depend on the results of the other ones.
func NewHandleFn(configProtocolQueue chan<- Entry, il integration.InstancesLookup, logger log.Entry) HandleFn {
	return func(cp protocol.ConfigProtocolV1) {
		//TODO trace logging
		// trace.CmdReq("received payload: %+v", crBatch)

		cr := &ctx.ConfigRequest{ConfigName: cp.ConfigName, ConfigHash: cp.Hash()}

		for _, ce := range cp.Config.Integrations {
			def, err := integration.NewDefinition(ce, il, nil, nil)
			def.ConfigRequest = cr
			if err != nil {
				logger.
					WithField("config_protocol_version", cp.ConfigProtocolVersion).
					WithField("name", cp.ConfigName).
					WithError(err).
					Warn("cannot create handler for config protocol")
				return
			}
			//trace.CmdReq("queued definition: %+v", def)
			configProtocolQueue <- Entry{def, databind.YAMLConfig{YAMLAgentConfig: cp.Config.Databind}}
		}
	}
}
