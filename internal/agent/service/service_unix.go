// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux || darwin
// +build linux darwin

package service

import (
	"os"
	"os/exec"

	"github.com/kardianos/service"

	"github.com/newrelic/infrastructure-agent/internal/os/api"
	"github.com/newrelic/infrastructure-agent/internal/os/api/signals"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

// Start is called when the service manager tells us to start
func (svc *Service) Start(s service.Service) (err error) {
	go svc.daemon.run(s)

	return
}

// Stop is called when the service manager commands us to stop
// By default systemD sends the SIGTERM signal to all the subprocesses, so the agent (child) process
// will have to handle it in order to property stop (otherwise it just gets killed 'immediately')
func (svc *Service) Stop(_ service.Service) (err error) {
	if svc.daemon.exited.Get() {
		return nil
	}

	log.Info("Service is stopping. waiting for agent process to terminate...")

	svc.daemon.Lock()
	defer svc.daemon.Unlock()

	svc.daemon.stopRequested.Set(true)

	err = svc.daemon.cmd.Process.Signal(signals.GracefulStop)
	if err != nil {
		log.WithError(err).Debug("Failed to send graceful stop signal to process.")
	}

	return svc.terminate(err)
}

// Shutdown is called in Windows only, when the machine is shutting down
func (svc *Service) Shutdown(_ service.Service) (err error) {
	// this is not being used in services other than Windows
	return nil
}

func (d *daemon) run(s service.Service) {
	for {
		d.Lock()
		d.cmd = exec.Command(GetCommandPath(d.args[0]), d.args[1:]...)
		d.cmd.Stdout = os.Stdout
		d.cmd.Stderr = os.Stderr
		d.Unlock()

		exitCode := api.CheckExitCode(d.cmd.Run())

		switch exitCode {
		case api.ExitCodeRestart:
			log.Info("child process requested restart")
			continue
		default:
			d.exitWithChildStatus(s, exitCode)
			return
		}
	}
}
