// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package cmdchannel

import (
	"context"
	"encoding/json"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/feature_flags"
	errors2 "github.com/pkg/errors"

	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/handler"
	"github.com/newrelic/infrastructure-agent/internal/agent/id"
	"github.com/newrelic/infrastructure-agent/pkg/backend/commandapi"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

var (
	ccsLogger = log.WithComponent("CommandChannelService")
)

// CmdHandleF command channel request handler function.
type CmdHandleF func(ctx context.Context, cmd commandapi.Command, initialFetch bool) (backoffSecs int, err error)

// CmdHandler handler for the a given command-channel command request.
type CmdHandler struct {
	Handle           CmdHandleF
	CmdName          string
	CmdArgumentsType interface{}
}

type srv struct {
	client            commandapi.Client
	pollDelaySecs     int
	handlersByCmdName map[string]*CmdHandler
	ffHandler         *handler.FFHandler // explicit to ease deps injection on runtime
}

func NewCmdHandler(cmdName string, cmdArgumentsType interface{}, handle CmdHandleF) *CmdHandler {
	return &CmdHandler{
		CmdName:          cmdName,
		CmdArgumentsType: cmdArgumentsType,
		Handle:           handle,
	}
}

// NewService creates a service to poll and handle command channel commands.
func NewService(client commandapi.Client, config *config.Config, ffSetter feature_flags.Setter) Service {
	boHandle := func(ctx context.Context, cmd commandapi.Command, initialFetch bool) (backoffSecs int, err error) {
		var boArgs commandapi.BackoffArgs
		if err = json.Unmarshal(cmd.Args, &boArgs); err != nil {
			err = errors2.Wrap(handler.InvalidArgsErr, err.Error())
			return
		}
		backoffSecs = boArgs.Delay
		return
	}

	ffHandler := handler.NewFFHandler(config, ffSetter, log.WithComponent("FFHandler"))

	handlers := map[string]*CmdHandler{
		"backoff_command_channel": NewCmdHandler("backoff_command_channel", nil, boHandle),
		"set_feature_flag":        NewCmdHandler("set_feature_flag", nil, ffHandler.Handle),
	}

	return &srv{
		client:            client,
		pollDelaySecs:     config.CommandChannelIntervalSec,
		ffHandler:         ffHandler,
		handlersByCmdName: handlers,
	}
}

// InitialFetch initial poll to command channel
func (s *srv) InitialFetch(ctx context.Context) (InitialCmdResponse, error) {
	cmds, err := s.client.GetCommands(entity.EmptyID)
	if err != nil {
		return InitialCmdResponse{}, err
	}

	for _, cmd := range cmds {
		s.handle(ctx, cmd, true)
	}

	return InitialCmdResponse{
		Ts:    time.Now(),
		Delay: time.Duration(s.pollDelaySecs) * time.Second,
	}, nil
}

// Run polls command channel periodically, in case 1st poll returned a delay, it starts afterwards.
func (s *srv) Run(ctx context.Context, agentIDProvide id.Provide, initialRes InitialCmdResponse) {
	d := initialRes.Delay - time.Now().Sub(initialRes.Ts)
	if d <= 0 {
		d = s.nextPollInterval()
	}

	t := time.NewTicker(d)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			cmds, err := s.client.GetCommands(agentIDProvide().ID)
			if err != nil {
				ccsLogger.WithError(err).Warn("commands poll failed")
			} else {
				for _, cmd := range cmds {
					s.handle(ctx, cmd, false)
				}
			}
			t.Stop()
			t = time.NewTicker(s.nextPollInterval())
		}
	}
}

// SetOHIHandler injects the handler dependency. A proper refactor of agent services injection will
// be required for this to be injected via srv constructor.
func (s *srv) SetOHIHandler(h handler.OHIEnabler) {
	s.ffHandler.SetOHIHandler(h)
}

func (s *srv) nextPollInterval() time.Duration {
	if s.pollDelaySecs <= 0 {
		s.pollDelaySecs = 1
	}
	return time.Duration(s.pollDelaySecs) * time.Second
}

func (s *srv) handle(ctx context.Context, c commandapi.Command, initialFetch bool) {
	h, ok := s.handlersByCmdName[c.Name]
	if !ok {
		ccsLogger.
			WithField("cmd_id", c.ID).
			WithField("cmd_name", c.Name).
			Error("no handler for command-channel cmd")
		return
	}

	backoffSecs, err := h.Handle(ctx, c, initialFetch)
	if err != nil {
		ccsLogger.
			WithField("cmd_id", c.ID).
			WithField("cmd_name", c.Name).
			WithField("cmd_arguments", c.Args).
			WithError(err).
			Error("error handling cmd-channel request")

	}
	if backoffSecs > 0 {
		s.pollDelaySecs = backoffSecs
	}
}
