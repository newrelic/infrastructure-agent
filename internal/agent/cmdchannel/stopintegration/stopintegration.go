// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package stopintegration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel"
	"github.com/newrelic/infrastructure-agent/pkg/backend/commandapi"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/shirou/gopsutil/process"
)

const (
	terminationGracePeriod = 1 * time.Minute
)

// Errors
var (
	ErrNoIntPID = errors.New("missing required \"pid\"")
)

type runIntArgs struct {
	PID             int      `json:"pid"`
	IntegrationName string   `json:"integration_name"`
	IntegrationArgs []string `json:"integration_args"`
}

// NewHandler creates a cmd-channel handler for stop-integration requests.
func NewHandler(logger log.Entry) *cmdchannel.CmdHandler {
	handleF := func(ctx context.Context, cmd commandapi.Command, initialFetch bool) (err error) {
		if runtime.GOOS == "windows" {
			return cmdchannel.ErrOSNotSupported
		}

		var args runIntArgs
		if err = json.Unmarshal(cmd.Args, &args); err != nil {
			err = cmdchannel.NewArgsErr(err)
			return
		}

		if args.PID == 0 {
			err = cmdchannel.NewArgsErr(ErrNoIntPID)
			return
		}

		p, err := process.NewProcess(int32(args.PID))
		if err != nil {
			logDecorated(logger, cmd, args, err).Warn("cannot retrieve process")
			return
		}

		err = p.TerminateWithContext(ctx)
		if err != nil {
			logDecorated(logger, cmd, args, err).Debug("cannot SIGTERM process")
		} else {
			// wait grace period, blocking is fine as cmd handlers run within their own goroutines.
			time.Sleep(terminationGracePeriod)
		}

		isRunning, err := p.IsRunningWithContext(ctx)
		if err != nil {
			logDecorated(logger, cmd, args, err).Warn("cannot retrieve process running state")
		}

		if isRunning {
			if err = p.KillWithContext(ctx); err != nil {
				logDecorated(logger, cmd, args, err).Error("cannot kill process")
			}
		}

		// no further error handling required
		err = nil

		return
	}

	return cmdchannel.NewCmdHandler("run_integration", handleF)
}

func logDecorated(logger log.Entry, cmd commandapi.Command, args runIntArgs, err error) log.Entry {
	return logger.
		WithField("cmd_id", cmd.ID).
		WithField("cmd_name", cmd.Name).
		WithField("cmd_args", string(cmd.Args)).
		WithField("cmd_args_pid", args.PID).
		WithField("cmd_args_name", args.IntegrationName).
		WithField("cmd_args_args", fmt.Sprintf("%+v", args.IntegrationArgs)).
		WithError(err)
}
