package cmdrequest

import (
	"context"
	"errors"
	"fmt"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/cmdrequest/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/config"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

var (
	NoopHandleFn    = func(context.Context, protocol.CmdRequestV1) {}
	ErrNotSupported = errors.New("integration instance lookup not supported for cmd request handler")
	noLookup        = integration.InstancesLookup{
		Legacy: func(_ integration.DefinitionCommandConfig) (integration.Definition, error) {
			return integration.Definition{}, ErrNotSupported
		},
		ByName: func(_ string) (string, error) {
			return "", ErrNotSupported
		},
	}
)

type HandleFn func(context.Context, protocol.CmdRequestV1)

// NewHandleFn creates a handler func that runs every command within the request batch independently.
// Each command is run in parallel and won't depend on the results of the other ones.
func NewHandleFn(definitionQueue chan<- integration.Definition, logger log.Entry) HandleFn {
	return func(ctx context.Context, crBatch protocol.CmdRequestV1) {
		for _, c := range crBatch.Commands {

			def, err := integration.NewDefinition(NewConfigFromCmdReq(c), noLookup, nil, nil)
			if err != nil {
				logger.
					WithField("cmd_req_version", crBatch.CommandRequestVersion).
					WithField("name", c.Name).
					WithField("command", c.Command).
					WithField("args", fmt.Sprintf("%+v", c.Args)).
					WithField("env", fmt.Sprintf("%+v", c.Env)).
					Warn("cannot create handler for cmd request")
				return
			}

			definitionQueue <- def
		}
	}
}

// NewConfigFromCmdReq creates an integration config from a command request.
// TODO this still needs to be worked on
func NewConfigFromCmdReq(cr protocol.CmdRequestV1Cmd) config.ConfigEntry {
	return config.ConfigEntry{
		Name:    cr.Name,
		Command: cr.Command,
		//Arguments: cr.Args, // TODO fix: Arguments are converted into key-value env-vars
		Env: cr.Env,
	}
}
