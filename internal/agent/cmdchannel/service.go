// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package cmdchannel

import (
	context2 "context"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/feature_flags"

	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/handler"
	"github.com/newrelic/infrastructure-agent/internal/agent/id"
	"github.com/newrelic/infrastructure-agent/pkg/backend/commandapi"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

var ccsLogger = log.WithComponent("CommandChannelService")

type srv struct {
	client        commandapi.Client
	config        *config.Config
	pollDelaySecs int
	ffHandler     *handler.FFHandler
}

// NewService creates a service to poll and handle command channel commands.
func NewService(client commandapi.Client, config *config.Config, ffSetter feature_flags.Setter) Service {
	return &srv{
		client:        client,
		config:        config,
		pollDelaySecs: config.CommandChannelIntervalSec,
		ffHandler:     handler.NewFFHandler(config, ffSetter),
	}
}

// InitialFetch initial poll to command channel
func (s *srv) InitialFetch() (InitialCmdResponse, error) {
	cmds, err := s.client.GetCommands(entity.EmptyID)
	if err != nil {
		return InitialCmdResponse{}, err
	}

	for _, cmd := range cmds {
		s.handle(cmd, true)
	}

	return InitialCmdResponse{
		Ts:    time.Now(),
		Delay: time.Duration(s.pollDelaySecs) * time.Second,
	}, nil
}

// Run polls command channel periodically, in case 1st poll returned a delay, it starts afterwards.
func (s *srv) Run(ctx context2.Context, agentIDProvide id.Provide, initialRes InitialCmdResponse) {
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
					s.handle(cmd, false)
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

func (s *srv) handle(c commandapi.Command, initialFetch bool) {
	switch c.Args.(type) {
	case commandapi.FFArgs:
		ffArgs := c.Args.(commandapi.FFArgs)
		s.ffHandler.Handle(ffArgs, initialFetch)
	case commandapi.BackoffArgs:
		boArgs := c.Args.(commandapi.BackoffArgs)
		s.pollDelaySecs = boArgs.Delay
	}
}
