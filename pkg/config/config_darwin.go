// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package config

import (
	"bytes"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/kelseyhightower/envconfig"
)

const (
	defaultConnectEnabled = true
)

func init() {
	defaultConfigFiles = []string{
		"newrelic-infra.yml",
		filepath.Join("/etc", "newrelic-infra.yml"),
		filepath.Join("/etc", "newrelic-infra", "newrelic-infra.yml"),
	}
	defaultPluginConfigFiles = []string{
		filepath.Join("/etc", "newrelic-infra-plugins.yml"),
		filepath.Join("/etc", "newrelic-infra", "newrelic-infra-plugins.yml"),
	}
	defaultPluginInstanceDir = filepath.Join("/etc", "newrelic-infra", "integrations.d")
	defaultConfigDir = filepath.Join("/etc", "newrelic-infra")

	defaultAgentDir = filepath.Join("/var", "db", "newrelic-infra")
	defaultLogFile = filepath.Join("/var", "db", "newrelic-infra", "newrelic-infra.log")
	defaultNetworkInterfaceFilters = map[string][]string{
		"prefix":  {"dummy", "lo", "vmnet", "sit", "tun", "tap", "veth"},
		"index-1": {"tun", "tap"},
	}

	defaultLoggingBinDir = "logging"
	defaultLoggingConfigsDir = "logging.d"
	defaultFluentBitExe = "fluent-bit"
	defaultFluentBitParsers = "parsers.conf"
	defaultFluentBitNRLib = "out_newrelic.so"

	// this is the default dir the infra sdk uses to store "temporary" data
	defaultIntegrationsTempDir = filepath.Join("/tmp", "nr-integrations")
}

func configOverride(cfg *Config) {
	if err := envconfig.Process(envPrefix, cfg); err != nil {
		clog.WithError(err).Error("unable to interpret environment variables")
	}
	hostOverride(cfg)
}

func hostOverride(cfg *Config) {
	var prefix string
	if cfg.OverrideHostRoot != "" {
		prefix = cfg.OverrideHostRoot
		_ = os.Setenv("HOST_PROC", filepath.Join(prefix, "/proc"))
		_ = os.Setenv("HOST_SYS", filepath.Join(prefix, "/sys"))
		_ = os.Setenv("HOST_ETC", filepath.Join(prefix, "/etc"))
		_ = os.Setenv("HOST_VAR", filepath.Join(prefix, "/var"))
	}
	if cfg.OverrideHostProc != "" {
		cfg.OverrideHostProc = prefix + cfg.OverrideHostProc
		_ = os.Setenv("HOST_PROC", cfg.OverrideHostProc)
	}
	if cfg.OverrideHostSys != "" {
		cfg.OverrideHostSys = prefix + cfg.OverrideHostSys
		_ = os.Setenv("HOST_SYS", cfg.OverrideHostSys)
	}
	if cfg.OverrideHostEtc != "" {
		cfg.OverrideHostEtc = prefix + cfg.OverrideHostEtc
		_ = os.Setenv("HOST_ETC", cfg.OverrideHostEtc)
	}
}

// runtimeValues returns runtime loaded values: like executable path, agent running mode and user, being mode:
// - root if the running user is root
// - privileged if the binary has capabilities: `cap_dac_read_search` and `cap_sys_ptrace`.
// - unprivileged otherwise
func runtimeValues() (agentMode, agentUser, executablePath string) {
	agentMode = ModeUnknown

	usr, err := user.Current()
	if err != nil {
		clog.WithError(err).Warn("unable to fetch current user")
	}
	if usr != nil {
		agentUser = usr.Username

		if usr.Uid == "0" || usr.Username == "root" {
			agentMode = ModeRoot
			return
		}
	}

	executablePath, err = os.Executable()
	if err != nil {
		clog.WithError(err).Warn("unable to fetch the agent executable path")
		agentMode = ModeUnprivileged
		return
	}

	output, err := exec.Command(getCapPath(), executablePath).Output()
	if err != nil {
		clog.WithError(err).Debug("Cannot execute getcap command.")
		agentMode = ModeUnprivileged
		return
	}

	s := strings.ToLower(string(output))
	if strings.Contains(s, "cap_dac_read_search") && strings.Contains(s, "cap_sys_ptrace") {
		agentMode = ModePrivileged
		return
	}
	agentMode = ModeUnprivileged
	return
}

// getCapPath will return the path for getcap command.
func getCapPath() string {
	var getCap string

	output, err := exec.Command("sh", "-c", "command -v getcap").Output()
	if err != nil {
		clog.WithError(err).Debug("Cannot find getcap command location.")
	} else {
		getCap = string(bytes.TrimSpace(output))
	}

	// Check if the location is valid, otherwise we will use the default.
	_, err = os.Stat(getCap)

	if os.IsNotExist(err) {
		// Path for Debian 7
		getCap = "/sbin/getcap"
		clog.WithError(err).WithField("path", getCap).Debug("Using the default getcap command location.")
	}
	return getCap
}
