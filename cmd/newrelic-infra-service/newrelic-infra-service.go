// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:generate goversioninfo

package main

import (
	"os"

	"github.com/newrelic/infrastructure-agent/internal/agent/service"
	"github.com/newrelic/infrastructure-agent/internal/os/api"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

func main() {
	log.SetOutput(os.Stdout)
	log.Infof("Starting agent process: %s", service.GetCommandPath(os.Args[0]))

	// Anything logged before this point won't necessarily make it into the log
	// file that is configured on the preceding line.
	log.Info("Creating service...")

	// Create a native service wrapper for the agent and start it up.
	exitCodeC := make(chan int, 1)
	agentSvc, err := service.New(exitCodeC, os.Args...)

	if err != nil {
		log.WithError(err).Error("Initializing service manager support...")
		os.Exit(1)
	}

	if err = agentSvc.Run(); err != nil {
		log.WithError(err).Warn("Service exiting abnormally.")
	}

	err = service.WaitForExitOrTimeout(exitCodeC)
	if err == nil {
		return
	}

	// This might not be an error: child may have already exited.
	if errCode, ok := err.(*api.ExitCodeErr); ok {
		log.WithError(err).Warn("Service exiting with child process status code.")
		os.Exit(errCode.ExitCode())
	} else {
		log.WithError(err).Warn("Service exiting with child process .")
		os.Exit(1)
	}

	log.Info("Service exited.")
}
