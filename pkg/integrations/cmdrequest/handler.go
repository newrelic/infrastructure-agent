package cmdrequest

import (
	"fmt"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/cmdrequest/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/config"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/trace"
)

var (
	// helper for testing purposes
	NoopHandleFn = func(protocol.CmdRequestV1) {}
)

type HandleFn func(protocol.CmdRequestV1)

// NewHandleFn creates a handler func that runs every command within the request batch independently.
// Each command is run in parallel and won't depend on the results of the other ones.
func NewHandleFn(definitionQueue chan<- integration.Definition, il integration.InstancesLookup, logger log.Entry) HandleFn {
	return func(crBatch protocol.CmdRequestV1) {
		trace.CmdReq("received payload: %+v", crBatch)
		for _, c := range crBatch.Commands {

			def, err := integration.NewDefinition(newConfigFromCmdReq(c), il, nil, nil)
			if err != nil {
				logger.
					WithField("cmd_req_version", crBatch.CommandRequestVersion).
					WithField("name", c.Name).
					WithField("command", c.Command).
					WithField("args", fmt.Sprintf("%+v", c.Args)).
					WithField("env", fmt.Sprintf("%+v", c.Env)).
					WithError(err).
					Warn("cannot create handler for cmd request")
				return
			}

			trace.CmdReq("queued definition: %+v", def)
			definitionQueue <- def
		}
	}
}

// newConfigFromCmdReq creates an integration config from a command request.
func newConfigFromCmdReq(cr protocol.CmdRequestV1Cmd) config.ConfigEntry {
	// executable is provided
	if cr.Command != "" {
		return config.ConfigEntry{
			InstanceName: cr.Name,
			Exec:         append([]string{cr.Command}, cr.Args...),
			Env:          cr.Env,
			Interval:     "0",
		}

	}

	// executable would be looked up by integration name
	return config.ConfigEntry{
		InstanceName: cr.Name,
		CLIArgs:      cr.Args,
		Env:          cr.Env,
		Interval:     "0",
	}
}
