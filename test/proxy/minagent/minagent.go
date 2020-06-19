// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package minagent

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/sirupsen/logrus"
)

// Minimalistic agent for proxy integration testing

const DefaultConfig = "/fake-config.yml"

type ConfigOptions struct {
	ConfigFile  string            `json:"config_file"`
	Environment map[string]string `json:"environment"`
}

func Start(cfgOpt ConfigOptions) *exec.Cmd {
	malog := logrus.New().WithField("component", "minimal-agent-service")

	malog.Info("running new agent instance...")

	if cfgOpt.ConfigFile == "" {
		cfgOpt.ConfigFile = DefaultConfig
	}

	cmd := exec.Command("/agent", fmt.Sprintf("--config=%s", cfgOpt.ConfigFile))

	cmd.Env = make([]string, 0)
	// Set up current environment
	for k, v := range cfgOpt.Environment {
		val := fmt.Sprintf("%s=%s", k, v)
		malog.Infof("setting environment variable %s", val)
		cmd.Env = append(cmd.Env, val)
	}
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Start(); err != nil {
		malog.WithError(err).Error("executing agent process")
	}

	return cmd
}
