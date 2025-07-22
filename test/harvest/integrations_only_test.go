// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build harvest
// +build harvest

package harvest

import (
	"net/http"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/backend/inventoryapi"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/plugins"
	fixture "github.com/newrelic/infrastructure-agent/test/fixture/inventory"
	"github.com/newrelic/infrastructure-agent/test/infra"
	ihttp "github.com/newrelic/infrastructure-agent/test/infra/http"
	"github.com/stretchr/testify/assert"
)

func TestIntegrationsOnlyMode(t *testing.T) {
	t.Parallel()

	const timeout = 15 * time.Second

	testClient := ihttp.NewRequestRecorderClient()
	agt := infra.NewAgent(testClient.Client, func(config *config.Config) {
		config.DisplayName = "my_display_name"
		config.IsIntegrationsOnly = true
		config.HeartBeatSampleRate = 1
	})
	agt.Context.SetAgentIdentity(entity.Identity{ID: 10, GUID: "abcdef"})

	if err := plugins.RegisterPlugins(agt); err != nil {
		assert.FailNow(t, "fatal error while registering plugins")
	}
	go func() {
		_ = agt.Run()
	}()

	var req http.Request
	select {
	case req = <-testClient.RequestCh:
		agt.Terminate()
	case <-time.After(timeout):
		agt.Terminate()
		assert.FailNow(t, "timeout while waiting for a response")
	}

	fixture.AssertRequestContainsInventoryDeltas(t, req, []*inventoryapi.RawDelta{
		{
			Source:   "metadata/infra_agent",
			ID:       1,
			FullDiff: true,
			// Checking some common services that should exist in any linux host
			Diff: map[string]interface{}{
				"integrations_only": bool(true),
			},
		},
	})
}
