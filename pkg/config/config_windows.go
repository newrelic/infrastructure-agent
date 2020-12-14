// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package config

import (
	"os"
	"os/user"
	"path/filepath"

	"github.com/kelseyhightower/envconfig"
)

const (
	installationSubdir = "Program Files" + string(filepath.Separator) +
		"New Relic" + string(filepath.Separator) + "newrelic-infra"
	defaultAppDataSubDir  = "New Relic" + string(filepath.Separator) + "newrelic-infra"
	defaultConnectEnabled = true
)

func init() {
	defaultNetworkInterfaceFilters = map[string][]string{
		"prefix": {"Loop", "isatap"},
	}
	// NOTE: On Windows, we need at least ComSpec for the agent to be able to run batch files
	// and SystemRoot for the integration to be able to use the networking layer in some libraries.
	defaultPassthroughEnvironment = []string{"ComSpec", "SystemRoot"}

	programData := os.Getenv("ProgramData")
	defaultAppDataDir = filepath.Join(programData, defaultAppDataSubDir)

	sysDrive := os.Getenv("SystemDrive") + string(filepath.Separator)

	defaultAgentDir = filepath.Join(sysDrive, installationSubdir)
	defaultConfigDir = defaultAgentDir
	defaultLogFile = filepath.Join(defaultAgentDir, "newrelic-infra.log")
	defaultPluginInstanceDir = filepath.Join(defaultAgentDir, "integrations.d")

	defaultConfigFiles = []string{filepath.Join(defaultAgentDir, "newrelic-infra.yml")}
	defaultPluginConfigFiles = []string{filepath.Join(defaultAgentDir, "newrelic-infra-plugins.yml")}

	defaultLoggingBinDir = "logging"
	defaultLoggingConfigsDir = "logging.d"

	defaultFluentBitExe = "fluent-bit.exe"
	defaultFluentBitParsers = "parsers.conf"
	defaultFluentBitNRLib = "out_newrelic.dll"
}

func runtimeValues() (userMode, agentUser, executablePath string) {
	userMode = ModeRoot

	usr, err := user.Current()
	if err != nil {
		clog.WithError(err).Warn("unable to fetch current user")
	}
	if usr != nil {
		agentUser = usr.Username
	}

	executablePath, err = os.Executable()
	if err != nil {
		clog.WithError(err).Warn("unable to fetch the agent executable path")
	}

	return
}

func configOverride(cfg *Config) {
	if err := envconfig.Process(envPrefix, cfg); err != nil {
		clog.WithError(err).Error("unable to interpret environment variables")
	}
}
