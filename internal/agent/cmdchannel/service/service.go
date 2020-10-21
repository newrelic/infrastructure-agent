package service

import (
	"context"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel"
	"github.com/newrelic/infrastructure-agent/internal/agent/id"
	"github.com/newrelic/infrastructure-agent/pkg/backend/commandapi"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

var (
	ccsLogger = log.WithComponent("CommandChannelService")
)

type srv struct {
	pollDelaySecs     int
	client            commandapi.Client
	handlersByCmdName map[string]*cmdchannel.CmdHandler
}

// NewService creates a service to poll and handle command channel commands.
func NewService(client commandapi.Client, pollDelaySecs int, handlers ...*cmdchannel.CmdHandler) cmdchannel.Service {
	handlersByName := map[string]*cmdchannel.CmdHandler{}
	for _, h := range handlers {
		handlersByName[h.CmdName] = h
	}

	return &srv{
		client:            client,
		pollDelaySecs:     pollDelaySecs,
		handlersByCmdName: handlersByName,
	}
}

// InitialFetch initial poll to command channel
func (s *srv) InitialFetch(ctx context.Context) (cmdchannel.InitialCmdResponse, error) {
	cmds, err := s.client.GetCommands(entity.EmptyID)
	if err != nil {
		return cmdchannel.InitialCmdResponse{}, err
	}

	for _, cmd := range cmds {
		s.handle(ctx, cmd, true)
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
