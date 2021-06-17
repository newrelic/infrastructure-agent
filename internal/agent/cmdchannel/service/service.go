// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"sync"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel"
	"github.com/newrelic/infrastructure-agent/internal/agent/id"
	"github.com/newrelic/infrastructure-agent/pkg/backend/commandapi"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/trace"
)

const (
	handleBOTimeoutOnInitialFetch = 100 * time.Millisecond
)

var (
	ccsLogger = log.WithComponent("CommandChannelService")
)

type srv struct {
	pollDelaySecs     int
	pollDelaySecsC    <-chan int
	client            commandapi.Client
	handlersByCmdName map[string]*cmdchannel.CmdHandler
	acks              map[string]struct{} // command hashes successfully ack'd
	acksLock          sync.RWMutex
}

// NewService creates a service to poll and handle command channel commands.
func NewService(
	client commandapi.Client,
	pollDelaySecs int,
	backoffSecsC <-chan int,
	handlers ...*cmdchannel.CmdHandler) cmdchannel.Service {
	handlersByName := map[string]*cmdchannel.CmdHandler{}
	for _, h := range handlers {
		handlersByName[h.CmdName] = h
	}

	return &srv{
		client:            client,
		pollDelaySecs:     pollDelaySecs,
		pollDelaySecsC:    backoffSecsC,
		handlersByCmdName: handlersByName,
		acks:              make(map[string]struct{}),
		acksLock:          sync.RWMutex{},
	}
}

// InitialFetch initial poll to command channel
func (s *srv) InitialFetch(ctx context.Context) (cmdchannel.InitialCmdResponse, error) {
	cmds, err := s.client.GetCommands(entity.EmptyID)
	if err != nil {
		return cmdchannel.InitialCmdResponse{}, err
	}

	for _, cmd := range cmds {
		s.handle(ctx, cmd, true, entity.EmptyID)
	}

	select {
	case boSec := <-s.pollDelaySecsC:
		s.pollDelaySecs = boSec
	case <-time.NewTimer(handleBOTimeoutOnInitialFetch).C:
	}

	return cmdchannel.InitialCmdResponse{
		Ts:    time.Now(),
		Delay: time.Duration(s.pollDelaySecs) * time.Second,
	}, nil
}

// Run polls command channel periodically, in case 1st poll returned a delay, it starts afterwards.
func (s *srv) Run(ctx context.Context, agentIDProvide id.Provide, initialRes cmdchannel.InitialCmdResponse) {
	d := initialRes.Delay - time.Now().Sub(initialRes.Ts)
	if d <= 0 {
		d = s.nextPollInterval()
	}

	t := time.NewTicker(d)
	for {
		select {
		case <-ctx.Done():
			return
		case boSecs := <-s.pollDelaySecsC:
			s.pollDelaySecs = boSecs
		case <-t.C:
			agentID := agentIDProvide().ID
			cmds, err := s.client.GetCommands(agentID)
			if err != nil {
				ccsLogger.WithError(err).Warn("commands poll failed")
			} else {
				for _, cmd := range cmds {
					s.handle(ctx, cmd, false, agentID)
				}
			}
			t.Stop()
			t = time.NewTicker(s.nextPollInterval())
		}
	}
}

func (s *srv) nextPollInterval() time.Duration {
	if s.pollDelaySecs <= 0 {
		s.pollDelaySecs = 1
	}
	return time.Duration(s.pollDelaySecs) * time.Second
}

func (s *srv) handle(ctx context.Context, c commandapi.Command, initialFetch bool, agentID entity.ID) {
	if s.wasACKed(c, agentID) {
		trace.CmdReq("skipping cmd already ACKed: %s, hash: %s, args: %s", c.Name, c.Hash, string(c.Args))
		return
	}

	if s.requiresAck(c, agentID) {
		trace.CmdReq("triggering ACK for cmd: %s, hash: %s, args: %s", c.Name, c.Hash, string(c.Args))
		if err := s.ack(agentID, c); err != nil {
			ccsLogger.
				WithField("cmd_hash", c.Hash).
				WithField("cmd_name", c.Name).
				WithField("cmd_args", string(c.Args)).
				WithError(err).
				Error("cannot ACK command")
		}
	}

	if s.notReadyToHandle(c, agentID) {
		// discarding commands is safe, they will be requested again
		return
	}

	h, ok := s.handlersByCmdName[c.Name]
	if !ok {
		ccsLogger.
			WithField("cmd_hash", c.Hash).
			WithField("cmd_name", c.Name).
			Error("no handler for command-channel cmd")
		return
	}

	// TODO add concurrency support to FF handlers
	if c.Name != "set_feature_flag" {
		go func() {
			s.handleWrap(h, ctx, c, initialFetch)
		}()
	} else {
		s.handleWrap(h, ctx, c, initialFetch)
	}
}

func (s *srv) handleWrap(h *cmdchannel.CmdHandler, ctx context.Context, c commandapi.Command, initialFetch bool) {
	err := h.Handle(ctx, c, initialFetch)
	if err != nil {
		ccsLogger.
			WithField("cmd_hash", c.Hash).
			WithField("cmd_name", c.Name).
			WithField("cmd_args", string(c.Args)).
			WithError(err).
			Error("error handling cmd-channel request")

	}
}

func (s *srv) notReadyToHandle(c commandapi.Command, agentID entity.ID) bool {
	return c.ID != 0 && agentID.IsEmpty()
}

func (s *srv) wasACKed(c commandapi.Command, agentID entity.ID) bool {
	if c.Hash == "" {
		return false
	}

	_, ok := s.acks[c.Hash]
	return ok
}

func (s *srv) requiresAck(c commandapi.Command, agentID entity.ID) bool {
	return c.Hash != "" && !agentID.IsEmpty()
}

func (s *srv) ack(agentID entity.ID, c commandapi.Command) error {
	err := s.client.AckCommand(agentID, c.Hash)
	if err == nil {
		s.acks[c.Hash] = struct{}{}
	}
	return err
}
