// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package runintegration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/backend/commandapi"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/config"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

// Errors
var (
	ErrNoIntName = errors.New("missing required \"integration_name\"")
)

type runIntArgs struct {
	IntegrationName string   `json:"integration_name"`
	IntegrationArgs []string `json:"integration_args"`
}

// NewHandler creates a cmd-channel handler for run-integration requests.
func NewHandler(definitionQ chan<- integration.Definition, il integration.InstancesLookup, logger log.Entry) *cmdchannel.CmdHandler {
	handleF := func(ctx context.Context, cmd commandapi.Command, initialFetch bool) (err error) {
		var args runIntArgs
		if err = json.Unmarshal(cmd.Args, &args); err != nil {
			err = cmdchannel.NewArgsErr(err)
			return
		}

		if args.IntegrationName == "" {
			err = cmdchannel.NewArgsErr(ErrNoIntName)
			return
		}

		def, err := integration.NewDefinition(newConfigFromCmdChannelRunInt(args), il, nil, nil)
		if err != nil {
			logger.
				WithField("cmd_id", cmd.ID).
				WithField("cmd_name", cmd.Name).
				WithField("cmd_args", string(cmd.Args)).
				WithField("cmd_args_name", args.IntegrationName).
				WithField("cmd_args_args", fmt.Sprintf("%+v", args.IntegrationArgs)).
				WithError(err).
				Warn("cannot create handler for cmd channel run_integration requests")
			return
		}

		definitionQ <- def
		return
	}

	return cmdchannel.NewCmdHandler("run_integration", handleF)
}

// newConfigFromCmdReq creates an integration config from a command request.
func newConfigFromCmdChannelRunInt(args runIntArgs) config.ConfigEntry {
	// executable would be looked up by integration name
	return config.ConfigEntry{
		InstanceName: args.IntegrationName,
		CLIArgs:      args.IntegrationArgs,
	}
}
