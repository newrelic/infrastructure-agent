// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//nolint:exhaustruct
package main

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

func Test_configureLogRedirection(t *testing.T) {
	// Given a new MemLogger with data
	l := log.NewMemLogger(ioutil.Discard)
	_, err := l.Write([]byte("example logs here"))
	require.NoError(t, err)

	// And a log file
	logFile, err := ioutil.TempFile("", "newLogs.txt")
	require.NoError(t, err)

	// default log config
	logConf := config.NewLogConfig()
	logConf.File = logFile.Name()

	// When log redirection is configured to log file
	assert.True(t, configureLogRedirection(logConf, l))

	// Then data previously stored in MemLogger gets written into log file
	dat, err := ioutil.ReadFile(logFile.Name())
	require.NoError(t, err)
	assert.Equal(t, "example logs here", string(dat))
}

func Test_getPluginSourceDirs(t *testing.T) {
	t.Parallel()

	const (
		safeBinDir = "/opt/newrelic-infra"
		agentDir   = "/var/db/newrelic-infra"
		customDir  = "/custom/integrations"
	)

	// custom_plugin_installation_dir is the only source dir kept when the flag is set.
	// Default source dirs under safe_bin_dir and agent_dir are skipped when the flag is set.
	safeBinNRI := filepath.Join(safeBinDir, config.DefaultIntegrationsDir)
	safeBinCustom := filepath.Join(safeBinDir, "custom-integrations")
	agentCustom := filepath.Join(agentDir, "custom-integrations")
	agentNRI := filepath.Join(agentDir, config.DefaultIntegrationsDir)
	agentBundled := filepath.Join(agentDir, "bundled-plugins")
	agentPlugins := filepath.Join(agentDir, "plugins")

	cases := []struct {
		name            string
		disableScan     bool
		wantContains    []string
		wantNotContains []string
	}{
		{
			name:        "standalone mode scans the default integration dirs",
			disableScan: false,
			wantContains: []string{
				safeBinNRI, safeBinCustom, customDir,
				agentCustom, agentNRI, agentBundled, agentPlugins,
			},
		},
		{
			name:        "flag enabled scans only custom_plugin_installation_dir",
			disableScan: true,
			wantContains: []string{
				customDir,
			},
			wantNotContains: []string{
				safeBinNRI, safeBinCustom,
				agentCustom, agentNRI, agentBundled, agentPlugins,
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{
				SafeBinDir:                  safeBinDir,
				AgentDir:                    agentDir,
				CustomPluginInstallationDir: customDir,
				DisablePluginDefaultDirScan: testCase.disableScan,
			}

			got := getPluginSourceDirs(cfg)

			for _, dir := range testCase.wantContains {
				assert.Contains(t, got, dir)
			}

			for _, dir := range testCase.wantNotContains {
				assert.NotContains(t, got, dir)
			}
		})
	}
}
