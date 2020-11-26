// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package stopintegration

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel"
	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/runintegration"
	"github.com/newrelic/infrastructure-agent/pkg/backend/commandapi"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/stoppable"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/trace"
	"github.com/shirou/gopsutil/process"
)

const (
	terminationGracePeriod = 1 * time.Minute
)

// NewHandler creates a cmd-channel handler for stop-integration requests.
func NewHandler(tracker *stoppable.Tracker, l log.Entry) *cmdchannel.CmdHandler {
	handleF := func(ctx context.Context, cmd commandapi.Command, initialFetch bool) (err error) {
		if runtime.GOOS == "windows" {
			return cmdchannel.ErrOSNotSupported
		}

		trace.CmdReq("stop integration request received")

		var args runintegration.RunIntArgs
		if err = json.Unmarshal(cmd.Args, &args); err != nil {
			err = cmdchannel.NewArgsErr(err)
			return
		}

		if args.IntegrationName == "" {
			err = cmdchannel.NewArgsErr(runintegration.ErrNoIntName)
			return
		}

		pidC, tracked := tracker.PIDReadChan(args.Hash())

		// integration isn't running
		if pidC == nil {
			if tracked {
				logDecorated(l, cmd, args).Debug("integration is not running")
			} else {
				logDecorated(l, cmd, args).Warn("cannot stop non tracked integration")
			}
			return nil
		}

		p, err := process.NewProcess(int32(<-pidC))
		if err != nil {
			logDecorated(l, cmd, args).WithError(err).Warn("cannot retrieve process")
			return
		}

		// request graceful stop (SIGTERM)
		err = p.TerminateWithContext(ctx)
		if err != nil {
			logDecorated(l, cmd, args).WithError(err).Debug("cannot SIGTERM process")
		} else {
			// wait grace period, blocking is fine as cmd handlers run within their own goroutines.
			time.Sleep(terminationGracePeriod)
		}

		isRunning, err := p.IsRunningWithContext(ctx)
		if err != nil {
			logDecorated(l, cmd, args).WithError(err).Warn("cannot retrieve process running state")
		}

		// force termination (SIGKILL)
		if isRunning || err != nil {
			stopped := tracker.Kill(args.Hash())
			logDecorated(l, cmd, args).WithField("stopped", stopped).Debug("integration force quit")
		}

		// no further error handling required
		err = nil

		return
	}

	return cmdchannel.NewCmdHandler("run_integration", handleF)
}

func logDecorated(logger log.Entry, cmd commandapi.Command, args runintegration.RunIntArgs) log.Entry {
	return logger.
		WithField("cmd_id", cmd.ID).
		WithField("cmd_name", cmd.Name).
		WithField("cmd_args", string(cmd.Args)).
		WithField("cmd_args_name", args.IntegrationName).
		WithField("cmd_args_args", fmt.Sprintf("%+v", args.IntegrationArgs))
}
