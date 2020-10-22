// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package backoff

import (
	"context"
	"encoding/json"

	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel"
	"github.com/newrelic/infrastructure-agent/pkg/backend/commandapi"
)

type args struct {
	DelaySecs int `json:"delay"`
}

// NewHandler creates a cmd-channel handler for cmd poll backoff requests.
func NewHandler() *cmdchannel.CmdHandler {
	handleF := func(ctx context.Context, cmd commandapi.Command, initialFetch bool) (backoffSecs int, err error) {
		var boArgs args
		if err = json.Unmarshal(cmd.Args, &boArgs); err != nil {
			err = cmdchannel.NewArgsErr(err)
			return
		}
		backoffSecs = boArgs.DelaySecs
		return
	}

	return cmdchannel.NewCmdHandler("backoff_command_channel", handleF)
}
