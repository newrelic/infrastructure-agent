package backoff

import (
	"context"
	"encoding/json"

	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel"
	"github.com/newrelic/infrastructure-agent/pkg/backend/commandapi"
	errors2 "github.com/pkg/errors"
)

type args struct {
	DelaySecs int `json:"delay"`
}

// NewHandler creates a cmd-channel handler for cmd poll backoff requests.
func NewHandler() *cmdchannel.CmdHandler {
	handleF := func(ctx context.Context, cmd commandapi.Command, initialFetch bool) (backoffSecs int, err error) {
		var boArgs args
		if err = json.Unmarshal(cmd.Args, &boArgs); err != nil {
			err = errors2.Wrap(cmdchannel.InvalidArgsErr, err.Error())
			return
		}
		backoffSecs = boArgs.DelaySecs
		return
	}

	return cmdchannel.NewCmdHandler("backoff_command_channel", handleF)
}
