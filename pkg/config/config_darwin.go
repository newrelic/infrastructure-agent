// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package config

import (
	"github.com/kelseyhightower/envconfig"
)

const (
	defaultConnectEnabled = true
)

func init() { //nolint:gochecknoinits
	// add PATH environment variable to all integrations
	defaultPassthroughEnvironment = []string{"PATH"}
}

func runtimeValues() (userMode, agentUser, executablePath string) {
	return ModeRoot, "", ""
}

func configOverride(cfg *Config) {
	if err := envconfig.Process(envPrefix, cfg); err != nil {
		clog.WithError(err).Error("unable to interpret environment variables")
	}
}

func loadDefaultLogRotation() LogRotateConfig {
	intPtr := func(a int) *int {
		return &a
	}
	return LogRotateConfig{
		MaxSizeMb:          intPtr(0),
		MaxFiles:           0,
		CompressionEnabled: false,
		FilePattern:        "",
	}
}
