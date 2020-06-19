// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package integration

import (
	"context"
	"io/ioutil"
	"runtime"
	"testing"

	config2 "github.com/newrelic/infrastructure-agent/pkg/integrations/v4/config"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/fixtures"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/testhelp"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/databind"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestConfigTemplate(t *testing.T) {
	// test suite setup
	file, err := ioutil.TempFile("", "config-template-test")
	require.NoError(t, err)
	_, err = file.Write([]byte("${discovery.ip}"))
	require.NoError(t, err)
	require.NoError(t, file.Close())

	type testCase struct {
		name   string
		config config2.ConfigEntry
	}
	cases := []testCase{{"Passing ${config.path} as command-line argument",
		config2.ConfigEntry{
			Name:         "test-integration",
			Exec:         testhelp.Command(fixtures.FileContentsWithArgCmd, "${config.path}"),
			TemplatePath: file.Name(),
		}}}
	if runtime.GOOS != "windows" { // executing Powershell passing env vars has problems
		cases = append(cases, testCase{"Using default CONFIG_PATH env var",
			config2.ConfigEntry{
				Name:         "test-integration",
				Exec:         testhelp.Command(fixtures.FileContentsCmd),
				TemplatePath: file.Name(),
			}})
		cases = append(cases, testCase{"Passing ${config.path} as environment variable",
			config2.ConfigEntry{
				Name:         "test-integration",
				Exec:         testhelp.Command(fixtures.FileContentsFromEnvCmd),
				Env:          map[string]string{"CUSTOM_CONFIG_PATH": "${config.path}"},
				TemplatePath: file.Name(),
			},
		})
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// GIVEN a template file containing discovery variables
			// That is loaded by an integration
			config, err := LoadConfigTemplate(tc.config.TemplatePath, tc.config.Config)
			require.NoError(t, err)

			i, err := New(tc.config, noLookup, nil, config)
			require.NoError(t, err)

			disc := databind.NewValues(nil,
				databind.NewDiscovery(data.Map{"discovery.ip": "1.2.3.4"}, data.InterfaceMap{"special": true, "label.important": "one"}, nil),
				databind.NewDiscovery(data.Map{"discovery.ip": "5.6.7.8"}, data.InterfaceMap{"special": false, "label.important": "two"}, nil),
			)

			// WHEN the integration is run
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			outputs, err := i.Run(ctx, &disc)
			require.NoError(t, err)

			// THEN the number of matches coincide with the discovered sources
			require.Len(t, outputs, 2)

			// AND the integration has correctly accepted the file templates with the discovery matches
			expectedIPs := map[string]struct{}{"1.2.3.4": {}, "5.6.7.8": {}}
			for _, out := range outputs {
				line := testhelp.ChannelRead(out.Output.Stdout)
				assert.Containsf(t, expectedIPs, line, "unexpected value: %v", line)
				delete(expectedIPs, line)
				assert.Len(t, out.ExtraLabels, 2)
				assert.Contains(t, out.ExtraLabels, "special")
				assert.Contains(t, out.ExtraLabels, "label.important")
			}
		})
	}
}

func TestEmbeddedConfig_String(t *testing.T) {
	type testCase struct {
		name   string
		config config2.ConfigEntry
	}
	cases := []testCase{{"Passing ${config.path} as command-line argument. External file embedded in yaml",
		config2.ConfigEntry{
			Name:   "test-integration",
			Exec:   testhelp.Command(fixtures.FileContentsWithArgCmd, "${config.path}"),
			Config: "${discovery.ip}",
		}}}
	if runtime.GOOS != "windows" { // executing Powershell passing env vars has problems
		cases = append(cases, testCase{"Using default CONFIG_PATH env var. External file embedded in yaml",
			config2.ConfigEntry{
				Name:   "test-integration",
				Exec:   testhelp.Command(fixtures.FileContentsCmd),
				Config: "${discovery.ip}",
			}})
		cases = append(cases, testCase{"Passing ${config.path} as environment variable. External file embedded in yaml",
			config2.ConfigEntry{
				Name:   "test-integration",
				Exec:   testhelp.Command(fixtures.FileContentsFromEnvCmd),
				Env:    map[string]string{"CUSTOM_CONFIG_PATH": "${config.path}"},
				Config: "${discovery.ip}",
			},
		})
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// GIVEN a template file containing discovery variables
			// That is loaded by an integration
			config, err := LoadConfigTemplate(tc.config.TemplatePath, tc.config.Config)
			require.NoError(t, err)
			i, err := New(tc.config, noLookup, nil, config)
			require.NoError(t, err)

			disc := databind.NewValues(nil,
				databind.NewDiscovery(data.Map{"discovery.ip": "1.2.3.4"}, nil, nil),
				databind.NewDiscovery(data.Map{"discovery.ip": "5.6.7.8"}, nil, nil),
			)

			// WHEN the integration is run
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			outputs, err := i.Run(ctx, &disc)
			require.NoError(t, err)

			// THEN the number of matches coincide with the discovered sources
			require.Len(t, outputs, 2)

			// AND the integration has correctly accepted the file templates with the discovery matches
			expectedIPs := map[string]struct{}{"1.2.3.4": {}, "5.6.7.8": {}}
			for _, out := range outputs {
				line := testhelp.ChannelRead(out.Output.Stdout)
				assert.Containsf(t, expectedIPs, line, "unexpected value: %q", line)
				delete(expectedIPs, line)
			}
		})
	}
}

func TestTimeout_Default(t *testing.T) {
	// GIVEN a configuration without timeout
	// WHEN an integration is loaded from it
	i, err := New(config2.ConfigEntry{Name: "foo", Exec: config2.ShlexOpt{"bar"}}, noLookup, nil, nil)
	require.NoError(t, err)

	// THEN the integration has a default timeout
	assert.Equal(t, defaultTimeout, i.Timeout)
}

func TestTimeout_TooLow(t *testing.T) {
	// GIVEN a configured timeout where the user forgot to write a suffix
	var config config2.ConfigEntry
	require.NoError(t, yaml.Unmarshal([]byte(`
name: foo
exec: bar
timeout: 40
`), &config))

	// WHEN the integration is loaded
	i, err := New(config, noLookup, nil, nil)
	require.NoError(t, err)

	// THEN the integration has the minimum allowed timeout
	assert.Equal(t, minimumTimeout, i.Timeout)
}

func TestTimeout_Disabled(t *testing.T) {
	// GIVEN a zero timeout value
	var config config2.ConfigEntry
	require.NoError(t, yaml.Unmarshal([]byte(`
name: foo
exec: bar
timeout: 0
`), &config))

	// WHEN the integration is loaded
	i, err := New(config, noLookup, nil, nil)
	require.NoError(t, err)

	// THEN the integration has a disabled timeout
	assert.False(t, i.TimeoutEnabled())
}
