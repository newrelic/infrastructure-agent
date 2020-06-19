// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package cmdchannel

import (
	"context"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/handler"
	"github.com/newrelic/infrastructure-agent/internal/agent/id"
)

type Service interface {
	InitialFetch() (InitialCmdResponse, error)
	Run(ctx context.Context, agentIDProvide id.Provide, initialRes InitialCmdResponse)
	SetOHIHandler(enabler handler.OHIEnabler)
}

// InitialCmdResponse initial command channel response.
type InitialCmdResponse struct {
	Ts    time.Time
	Delay time.Duration
}
