// Copyright 2024 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package plugins

import (
	"testing"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/test/infra"
	ihttp "github.com/newrelic/infrastructure-agent/test/infra/http"
	"github.com/stretchr/testify/assert"
)

//nolint:exhaustruct
func TestIsHeartbeatOnlyMode(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		cfg      *config.Config
		expected bool
	}{
		{
			"default_config",
			&config.Config{},
			false,
		},
		{
			"is_secure_forward_only",
			&config.Config{
				IsSecureForwardOnly: true,
			},
			true,
		},
		{
			"is_integrations_only",
			&config.Config{
				IsIntegrationsOnly: true,
			},
			true,
		},
	}
	for _, tc := range testCases {
		testCase := tc
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, testCase.expected, isHeartbeatOnlyMode(testCase.cfg))
		})
	}
}

func TestRegisterIntegrationsOnlyPlugin(t *testing.T) {
	t.Parallel()

	testClient := ihttp.NewRequestRecorderClient()

	testCases := []struct {
		name       string
		agt        *agent.Agent
		numPlugins int
	}{
		{
			"is_secure_forward_only_and_is_integrations_only",
			infra.NewAgent(testClient.Client, func(config *config.Config) {
				config.IsSecureForwardOnly = true
				config.IsIntegrationsOnly = true
			}),
			0,
		},
		{
			"is_integrations_only",
			infra.NewAgent(testClient.Client, func(config *config.Config) {
				config.IsIntegrationsOnly = true
			}),
			1,
		},
	}
	for _, tc := range testCases {
		testCase := tc
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			registerIntegrationsOnlyPlugin(testCase.agt)
			assert.Len(t, testCase.agt.Plugins(), testCase.numPlugins)
		})
	}
}
