// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux

package legacy

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/newrelic/infrastructure-agent/internal/testhelpers"
	"github.com/newrelic/infrastructure-agent/test/infra"
	infra2 "github.com/newrelic/infrastructure-agent/test/infra/http"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type pluginTestCase struct {
	integrationUser        string
	integrationUserSetting string
	expectedCmd            string
	expectedArgs           []string
}

func TestPluginV1InstanceIntegrationUser(t *testing.T) {
	testCases := []pluginTestCase{
		{
			integrationUser:        "root",
			integrationUserSetting: "integration_user: root",
			expectedCmd:            "/bin/sudo",
			expectedArgs: []string{
				"/bin/sudo", "-E", "-n", "-u", "root", "./bin/integration", "-inventory",
			},
		},
		{
			integrationUser:        "",
			integrationUserSetting: "",
			expectedCmd:            "./bin/integration",
			expectedArgs: []string{
				"./bin/integration", "-inventory",
			},
		},
	}

	configContent := `name: com.newrelic.integration
description: Test integration
protocol_version: 1
os: linux

commands:
    inventory:
        command:
          - ./bin/integration
          - -inventory
        prefix: config/integration
        interval: 60`

	instanceContent := `integration_name: com.newrelic.integration

instances:
    - name: integration-metrics
      command: inventory
      %s
      arguments:
          integration_argument: integration_value
      labels:
          label1: label1-value`

	for _, testCase := range testCases {
		t.Run(testCase.integrationUser, func(t *testing.T) {
			t.Logf(
				"with integrationUser == '%s' the command is %s and the arguments %v",
				testCase.integrationUser,
				testCase.expectedCmd,
				testCase.expectedArgs,
			)

			files := []testhelpers.MockFile{
				{
					ParentDir: "definitions",
					Name:      "definition.yml",
					Content:   configContent,
				},
				{
					ParentDir: "configs",
					Name:      "config.yml",
					Content:   fmt.Sprintf(instanceContent, testCase.integrationUserSetting),
				},
			}

			dir, err := testhelpers.NewMockDir(files)
			if err != nil {
				t.Fatal(err)
			}

			defer dir.Clear()

			definitionsDirs := []string{filepath.Join(dir.Path, "definitions")}
			configsDirs := []string{filepath.Join(dir.Path, "configs")}
			registry := NewPluginRegistry(definitionsDirs, configsDirs)
			assert.NotNil(t, registry)
			if err := registry.LoadPlugins(); err != nil {
				t.Fatal(err)
			}
			instance := registry.GetPluginInstances()[0]
			assert.Equal(t, testCase.integrationUser, instance.IntegrationUser)

			testClient := infra2.NewRequestRecorderClient()
			agent := infra.NewAgent(testClient.Client)
			runner := NewPluginRunner(registry, agent)
			if err := runner.ConfigureV1Plugins(agent.Context); err != nil {
				log.WithError(err).Debug("Can't configure new plugins.")
			}
			plugin, err := newExternalV1Plugin(runner, instance)
			require.Nil(t, err)

			pluginDir := plugin.pluginRunner.registry.GetPluginDir(plugin.pluginInstance.plugin)

			plugin.updateCmdWrappers(pluginDir)
			cmd := plugin.getCmdWrappers()
			require.Len(t, cmd, 1)

			assert.Equal(t, cmd[0].cmd.Path, testCase.expectedCmd)
			assert.Equal(t, cmd[0].cmd.Args, testCase.expectedArgs)
		})
	}
}
