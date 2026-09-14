// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cmdrequest

import (
	"fmt"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/cmdrequest/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/config"
	"github.com/newrelic/infrastructure-agent/pkg/log"

	agentConfig "github.com/newrelic/infrastructure-agent/pkg/config"
)

var (
	// helper for testing purposes
	//nolint:gochecknoglobals
	NoopHandleFn = func(protocol.CmdRequestV1, integration.Definition) {}
)

type HandleFn func(protocol.CmdRequestV1, integration.Definition)

// NewHandleFn creates a handler func that runs every command within the request batch independently.
// Each command is run in parallel and won't depend on the results of the other ones.
func NewHandleFn(definitionQueue chan<- integration.Definition, il integration.InstancesLookup, logger log.Entry) HandleFn {
	return func(crBatch protocol.CmdRequestV1, parentDefinition integration.Definition) {
		logger.WithField(agentConfig.TracesFieldName, agentConfig.FeatureTrace).Tracef("received payload: %+v", crBatch)
		for _, c := range crBatch.Commands {

			def, err := integration.NewDefinition(newConfigFromCmdReq(c, parentDefinition.ExecutorConfig.User), il, nil, nil)
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

			logger.WithField(agentConfig.TracesFieldName, agentConfig.FeatureTrace).Tracef("queued definition: %+v", def)
			definitionQueue <- def
		}
	}
}

// newConfigFromCmdReq creates an integration config from a command request.
// The child always inherits the parent integration's user so a restricted
// integration_user cannot be bypassed by a command request.
func newConfigFromCmdReq(cmdReq protocol.CmdRequestV1Cmd, parentUser string) config.ConfigEntry {
	// executable is provided
	if cmdReq.Command != "" {
		return config.ConfigEntry{
			InstanceName: cmdReq.Name,
			Exec:         append([]string{cmdReq.Command}, cmdReq.Args...),
			Env:          cmdReq.Env,
			Interval:     "0",
			User:         parentUser,
		}

	}

	// executable would be looked up by integration name
	return config.ConfigEntry{
		InstanceName: cmdReq.Name,
		CLIArgs:      cmdReq.Args,
		Env:          cmdReq.Env,
		Interval:     "0",
		User:         parentUser,
	}
}
