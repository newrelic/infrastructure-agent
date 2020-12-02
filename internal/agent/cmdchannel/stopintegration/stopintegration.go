// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package stopintegration

import (
	"context"
	"encoding/json"
	"runtime"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel"
	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/runintegration"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/backend/commandapi"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/stoppable"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/dm"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/trace"
	"github.com/shirou/gopsutil/process"
)

const (
	terminationGracePeriod = 1 * time.Minute
)

// NewHandler creates a cmd-channel handler for stop-integration requests.
func NewHandler(tracker *stoppable.Tracker, il integration.InstancesLookup, dmEmitter dm.Emitter, l log.Entry) *cmdchannel.CmdHandler {
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
				runintegration.LogDecorated(l, cmd, args).Debug("integration is not running")
			} else {
				runintegration.LogDecorated(l, cmd, args).Warn("cannot stop non tracked integration")
			}
			return nil
		}

		p, err := process.NewProcess(int32(<-pidC))
		if err != nil {
			runintegration.LogDecorated(l, cmd, args).WithError(err).Warn("cannot retrieve process")
			return
		}

		stopModeUsed := "error"
		// request graceful stop (SIGTERM)
		err = p.TerminateWithContext(ctx)
		if err != nil {
			runintegration.LogDecorated(l, cmd, args).WithError(err).Debug("cannot SIGTERM process")
		} else {
			// wait grace period, blocking is fine as cmd handlers run within their own goroutines.
			time.Sleep(terminationGracePeriod)
		}

		isRunning, err := p.IsRunningWithContext(ctx)
		if err != nil {
			runintegration.LogDecorated(l, cmd, args).WithError(err).Warn("cannot retrieve process running state")
		} else {
			stopModeUsed = "sigterm"
		}

		// force termination (SIGKILL)
		if isRunning || err != nil {
			stopped := tracker.Kill(args.Hash())
			runintegration.LogDecorated(l, cmd, args).WithField("stopped", stopped).Debug("integration force quit")
			stopModeUsed = "sigkill"
		}

		// notify platform
		if err = notifyPlatform(dmEmitter, il, cmd, args, stopModeUsed); err != nil {
			runintegration.LogDecorated(l, cmd, args).WithError(err).Warn("cannot notify platform about command")
		}

		// no further error handling required
		err = nil

		return
	}

	return cmdchannel.NewCmdHandler("stop_integration", handleF)
}

func notifyPlatform(dmEmitter dm.Emitter, il integration.InstancesLookup, cmd commandapi.Command, args runintegration.RunIntArgs, stopModeUsed string) error {
	def, err := integration.NewDefinition(runintegration.NewConfigFromCmdChannelRunInt(args), il, nil, nil)
	if err != nil {
		return err
	}

	def.CmdChannelHash = args.Hash()
	ev := cmd.Event(args.IntegrationName, args.IntegrationArgs)
	ev["cmd_stop_hash"] = args.Hash()
	ev["cmd_stop_mode"] = stopModeUsed
	runintegration.NotifyPlatform(dmEmitter, def, ev)

	return nil
}
