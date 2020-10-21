// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package cmdchannel

import (
	"context"
	"errors"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent/id"
	"github.com/newrelic/infrastructure-agent/pkg/backend/commandapi"
)

// Errors
var (
	InvalidArgsErr = errors.New("invalid arguments for command")
)

// CmdHandleF command channel request handler function.
type CmdHandleF func(ctx context.Context, cmd commandapi.Command, initialFetch bool) (backoffSecs int, err error)

// CmdHandler handler for the a given command-channel command request.
type CmdHandler struct {
	Handle  CmdHandleF
	CmdName string
}

// Service command channel service capable of handling command api cmd requests.
type Service interface {
	InitialFetch(ctx context.Context) (InitialCmdResponse, error)
	Run(ctx context.Context, agentIDProvide id.Provide, initialRes InitialCmdResponse)
}

// InitialCmdResponse initial command channel response.
type InitialCmdResponse struct {
	Ts    time.Time
	Delay time.Duration
}

// NewCmdHandler creates a command channel handler.
func NewCmdHandler(cmdName string, handle CmdHandleF) *CmdHandler {
	return &CmdHandler{
		CmdName: cmdName,
		Handle:  handle,
	}
}
