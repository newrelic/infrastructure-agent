// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:generate goversioninfo

package main

import (
	"context"
	"os"

	"github.com/newrelic/infrastructure-agent/pkg/log"
)

var buildVersion = "development"

func main() {
	log.SetOutput(os.Stdout)
	log.Info("Creating service...")

	ctx, cancel := context.WithCancel(context.Background())

	spvsr, err := newSupervisor(ctx, os.Args...)
	if err != nil {
		log.WithError(err).Error("cannot create supervisor")
		os.Exit(1)
	}
	if err := spvsr.run(); err != nil {
		cancel()
		log.WithError(err).Warn("Service exiting abnormally.")
	}
}
