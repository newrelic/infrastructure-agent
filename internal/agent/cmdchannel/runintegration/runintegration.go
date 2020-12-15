// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package runintegration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/backend/commandapi"
	"github.com/newrelic/infrastructure-agent/pkg/fwrequest"
	ctx2 "github.com/newrelic/infrastructure-agent/pkg/integrations/track/ctx"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/config"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/dm"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/trace"
)

const cmdName = "run_integration"

// Errors
var (
	ErrNoIntName = errors.New("missing required \"integration_name\"")
)

type RunIntArgs struct {
	IntegrationName string   `json:"integration_name"`
	IntegrationArgs []string `json:"integration_args"`
}

// Hash hashes the run-integration request, so intergation can be required to stop using same arguments.
func (a *RunIntArgs) Hash() string {
	return fmt.Sprintf("%s#%v", strings.TrimSpace(a.IntegrationName), a.IntegrationArgs)
}

// NewHandler creates a cmd-channel handler for run-integration requests.
func NewHandler(definitionQ chan<- integration.Definition, il integration.InstancesLookup, dmEmitter dm.Emitter, logger log.Entry) *cmdchannel.CmdHandler {
	handleF := func(ctx context.Context, cmd commandapi.Command, initialFetch bool) (err error) {
		trace.CmdReq("run integration request received")
		var args RunIntArgs
		if err = json.Unmarshal(cmd.Args, &args); err != nil {
			err = cmdchannel.NewArgsErr(err)
			return
		}

		if args.IntegrationName == "" {
			err = cmdchannel.NewArgsErr(ErrNoIntName)
			return
		}

		def, err := integration.NewDefinition(NewConfigFromCmdChannelRunInt(args), il, nil, nil)
		if err != nil {
			LogDecorated(logger, cmd, args).WithError(err).Warn("cannot create handler for cmd channel run_integration requests")
			return
		}

		cmdChanReq := ctx2.NewCmdChannelRequest(cmdName, args.Hash(), args.IntegrationName, args.IntegrationArgs, cmd.Metadata)
		def.CmdChanReq = &cmdChanReq

		definitionQ <- def

		ev := cmdChanReq.Event("cmd-api")

		NotifyPlatform(dmEmitter, def, ev)

		return
	}

	return cmdchannel.NewCmdHandler(cmdName, handleF)
}

func NotifyPlatform(dmEmitter dm.Emitter, def integration.Definition, ev protocol.EventData) {
	ds := protocol.NewEventDataset(time.Now().UnixNano(), ev)
	data := protocol.NewData("cmdapi.runintegration", "1", []protocol.Dataset{ds})
	dmEmitter.Send(fwrequest.NewFwRequest(def, nil, nil, data))
}

// newConfigFromCmdReq creates an integration config from a command request.
func NewConfigFromCmdChannelRunInt(args RunIntArgs) config.ConfigEntry {
	// executable would be looked up by integration name
	return config.ConfigEntry{
		InstanceName: args.IntegrationName,
		CLIArgs:      args.IntegrationArgs,
		Interval:     "0",
	}
}

func LogDecorated(logger log.Entry, cmd commandapi.Command, args RunIntArgs) log.Entry {
	return logger.
		WithField("cmd_id", cmd.ID).
		WithField("cmd_name", cmd.Name).
		WithField("cmd_args", string(cmd.Args)).
		WithField("cmd_args_name", args.IntegrationName).
		WithField("cmd_args_args", fmt.Sprintf("%+v", args.IntegrationArgs))
}
