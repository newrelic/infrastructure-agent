// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"os"
	"os/exec"

	"github.com/kardianos/service"

	"github.com/newrelic/infrastructure-agent/internal/os/api"
	"github.com/newrelic/infrastructure-agent/pkg/helpers/windows"
	"github.com/newrelic/infrastructure-agent/pkg/ipc"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

// Start starts the service
func (svc *Service) Start(s service.Service) (err error) {
	svc.daemon.wg.Add(1)
	go svc.daemon.run()
	return
}

// Stop stops the service
// There can be a condition where stop messages may not handled:
// - if we start and stop the service immediately, the agent may not stop properly
//   and so we have to kill it forcefully
func (svc *Service) Stop(s service.Service) (err error) {
	log.Info("service is stopping. notifying agent process...")

	svc.daemon.Lock()
	defer svc.daemon.Unlock()

	gracefulExit := make(chan struct{})
	go func() {
		svc.daemon.wg.Wait()
		close(gracefulExit)
	}()

	// notify the agent to gracefully stop
	windows.PostNotificationMessage(windows.PipeName, ipc.Stop)

	err = waitForExitOrTimeout(gracefulExit)
	if err == GracefulExitTimeoutErr {
		// the agent process did not exit in the allocated time.
		// make sure it doesn't stay around..
		svc.daemon.cmd.Process.Kill()
	}
	return err
}

// Shutdown stops the service whenever the machine is restarting or shutting down
// There can be a condition where shutdown messages may not handled:
// - if we start the service and shutdown the host immediately, the agent may not stop properly
//   and so we have to kill it forcefully
func (svc *Service) Shutdown(s service.Service) (err error) {
	log.Debug("Host is shutting down. notifying agent process.")

	svc.daemon.Lock()
	defer svc.daemon.Unlock()

	gracefulExit := make(chan struct{})
	go func() {
		svc.daemon.wg.Wait()
		close(gracefulExit)
	}()

	// notify the agent to update the shutdown status and then stop gracefully
	windows.PostNotificationMessage(windows.PipeName, ipc.Shutdown)

	err = waitForExitOrTimeout(gracefulExit)
	if err == GracefulExitTimeoutErr {
		// the agent process did not exit in the allocated time.
		// make sure it doesn't stay around..
		svc.daemon.cmd.Process.Kill()
	}
	return
}

func (d *daemon) run() {
	for {
		d.Lock()
		d.ctx, d.cancel = context.WithCancel(context.Background())
		d.cmd = exec.CommandContext(d.ctx, GetCommandPath(d.args[0]), d.args[1:]...)
		d.cmd.Stdout = os.Stdout
		d.cmd.Stderr = os.Stderr
		d.Unlock()

		exitCode := api.CheckExitCode(d.cmd.Run())

		switch exitCode {
		case api.ExitCodeRestart:
			log.Info("agent process exited with restart exit code. restarting agent process...")
			continue
		default:
			log.Info("agent process exited normally. stopping service...")
			d.wg.Done()
			return
		}
	}
}
