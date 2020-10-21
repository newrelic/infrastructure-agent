// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package runintegration

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel"
	"github.com/newrelic/infrastructure-agent/pkg/backend/commandapi"
)

// Errors
var (
	NoIntNameErr = errors.New("missing required \"integration_name\"")
)

type Args struct {
	IntegrationName string   `json:"integration_name"`
	IntegrationArgs []string `json:"integration_args"`
}

// NewHandler creates a cmd-channel handler for cmd poll backoff requests.
func NewHandler() *cmdchannel.CmdHandler {
	handleF := func(ctx context.Context, cmd commandapi.Command, initialFetch bool) (backoffSecs int, err error) {
		var args Args
		if err = json.Unmarshal(cmd.Args, &args); err != nil {
			err = cmdchannel.NewArgsErr(err)
			return
		}

		if args.IntegrationName == "" {
			err = cmdchannel.NewArgsErr(NoIntNameErr)
			return
		}

		return
	}

	return cmdchannel.NewCmdHandler("run_integration", handleF)
}
