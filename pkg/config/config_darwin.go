// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package config

import (
	"github.com/kelseyhightower/envconfig"
)

const (
	defaultConnectEnabled = true
)

func runtimeValues() (userMode, agentUser, executablePath string) {
	return ModeRoot, "", ""
}

func configOverride(cfg *Config) {
	if err := envconfig.Process(envPrefix, cfg); err != nil {
		clog.WithError(err).Error("unable to interpret environment variables")
	}
}
