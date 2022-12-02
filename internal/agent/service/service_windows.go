// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package service

import (
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
	go svc.daemon.run(s)
	return
}

// Stop stops the service
// There can be a condition where stop messages may not handled:
//   - if we start and stop the service immediately, the agent may not stop properly
//     and so we have to kill it forcefully
func (svc *Service) Stop(s service.Service) (err error) {
	if svc.daemon.exited.Get() {
		return nil
	}
	log.Info("service is stopping. notifying agent process.")

	svc.daemon.Lock()
	defer svc.daemon.Unlock()

	svc.daemon.stopRequested.Set(true)

	// notify the agent to gracefully stop
	windows.PostNotificationMessage(windows.GetPipeName(svcName), ipc.Stop)

	return svc.terminate(err)
}

// Shutdown stops the service whenever the machine is restarting or shutting down
// There can be a condition where shutdown messages may not handled:
//   - if we start the service and shutdown the host immediately, the agent may not stop properly
//     and so we have to kill it forcefully
func (svc *Service) Shutdown(s service.Service) (err error) {
	if svc.daemon.exited.Get() {
		return nil
	}
	log.Debug("host is shutting down. notifying agent process.")

	svc.daemon.Lock()
	defer svc.daemon.Unlock()

	svc.daemon.stopRequested.Set(true)

	// notify the agent to update the shutdown status and then stop gracefully
	windows.PostNotificationMessage(windows.GetPipeName(svcName), ipc.Shutdown)

	return svc.terminate(err)
}

func (d *daemon) run(s service.Service) {
	for {

		d.Lock()
		d.cmd = exec.Command(GetCommandPath(d.args[0]), d.args[1:]...)
		d.cmd.Stdout = os.Stdout
		d.cmd.Stderr = os.Stderr
		d.Unlock()

		exitCode := api.CheckExitCode(runAgentCmd(d.cmd))

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

// runAgentCmd will run the agent process and wait it to exit. The process will be added
// to an Windows job object to handle the child processes.
func runAgentCmd(cmd *exec.Cmd) error {
	jobObject, err := NewJob()
	if err != nil {
		log.Warnf("failed to create Job Object for Agent: %v", err)
	}
	defer func() {
		if jobObject != nil {
			if err := jobObject.Close(); err != nil {
				log.Warnf("failed to close Agent Job Object: %v", err)
			}
		}
	}()

	if err := cmd.Start(); err != nil {
		return err
	}

	if jobObject != nil {
		if err := jobObject.AddProcess(cmd.Process); err != nil {
			log.Warnf("failed to add Agent process to Job Object: %v", err)
		}
	}

	return cmd.Wait()
}
