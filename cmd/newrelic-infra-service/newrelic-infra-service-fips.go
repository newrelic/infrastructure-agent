// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:generate goversioninfo

//go:build fips

package main

import (
	_ "crypto/tls/fipsonly"
	"github.com/newrelic/infrastructure-agent/internal/agent/service"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"os"
)

func main() {
	log.SetOutput(os.Stdout)
	log.Info("Creating service...")

	// Create a native service wrapper for the agent and start it up.
	agentSvc, err := service.New(os.Args...)

	if err != nil {
		log.WithError(err).Error("Initializing service manager support...")
		os.Exit(1)
	}

	if err = agentSvc.Run(); err != nil {
		log.WithError(err).Warn("Service exiting abnormally.")
	}
}
