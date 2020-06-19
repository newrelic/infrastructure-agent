// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux darwin

package service

import (
	"context"
	"os"
	"os/exec"

	"github.com/kardianos/service"

	"github.com/newrelic/infrastructure-agent/internal/os/api"
	"github.com/newrelic/infrastructure-agent/internal/os/api/signals"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

// Start is called when the service manager tells us to start
func (svc *Service) Start(_ service.Service) (err error) {
	go svc.daemon.run()
	svc.daemon.wg.Add(1)

	return
}

// Stop is called when the service manager commands us to stop
// By default systemD sends the SIGTERM signal to all the subprocesses, so the agent (child) process
// will have to handle it in order to property stop (otherwise it just gets killed 'immediately')
func (svc *Service) Stop(_ service.Service) (err error) {
	log.Info("service is stopping. waiting for agent process to terminate...")

	gracefulExit := make(chan struct{})
	go func() {
		svc.daemon.wg.Wait()
		close(gracefulExit)
	}()

	err = svc.daemon.cmd.Process.Signal(signals.GracefulStop)
	if err != nil {
		log.WithError(err).Debug("Failed to send graceful stop signal to process.")
	}

	err = waitForExitOrTimeout(gracefulExit)
	if err == GracefulExitTimeoutErr {
		// the agent process did not exit in the allocated time.
		// make sure it doesn't stay around..
		svc.daemon.cmd.Process.Kill()
	}
	return
}

// Shutdown is called in Windows only, when the machine is shutting down
func (svc *Service) Shutdown(_ service.Service) (err error) {
	// this is not being used in services other than Windows
	return nil
}

func (d *daemon) run() {
	for {
		restart := make(chan struct{})
		d.ctx, d.cancel = context.WithCancel(context.Background())

		go func() {
			d.cmd = exec.CommandContext(d.ctx, GetCommandPath(d.args[0]), d.args[1:]...)
			d.cmd.Stdout = os.Stdout
			d.cmd.Stderr = os.Stderr

			exitCode := api.CheckExitCode(d.cmd.Run())

			switch exitCode {
			case api.ExitCodeRestart:
				log.Info("agent process requested restart")
				close(restart)
			default:
				log.WithField("exit_code", exitCode).
					Info("agent process exited, stopping agent service daemon...")
				d.cancel()
				d.wg.Done()
			}
		}()

		select {
		case <-d.ctx.Done():
			return
		case <-restart:
			continue
		}
	}
}
