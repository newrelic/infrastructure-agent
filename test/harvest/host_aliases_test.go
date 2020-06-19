// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build harvest

package harvest

import (
	"net/http"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"

	"github.com/newrelic/infrastructure-agent/pkg/backend/inventoryapi"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/plugins"
	fixture "github.com/newrelic/infrastructure-agent/test/fixture/inventory"
	"github.com/newrelic/infrastructure-agent/test/infra"
	ihttp "github.com/newrelic/infrastructure-agent/test/infra/http"
	"github.com/stretchr/testify/assert"
)

func TestHostAliases(t *testing.T) {
	const timeout = 5 * time.Second

	testClient := ihttp.NewRequestRecorderClient()
	a := infra.NewAgent(testClient.Client, func(config *config.Config) {
		config.DisplayName = "my_display_name"
	})

	cloudDetector := cloud.NewDetector(true, 0, 0, 0, false)
	a.RegisterPlugin(plugins.NewHostAliasesPlugin(a.Context, cloudDetector))

	go a.Run()

	var req http.Request
	select {
	case req = <-testClient.RequestCh:
		a.Terminate()
	case <-time.After(timeout):
		a.Terminate()
		assert.FailNow(t, "timeout while waiting for a response")
	}

	fixture.AssertRequestContainsInventoryDeltas(t, req, []*inventoryapi.RawDelta{
		{
			Source:   "metadata/host_aliases",
			ID:       1,
			FullDiff: true,
			// Checking some common /proc/sys entries that should exist in any linux host
			Diff: map[string]interface{}{
				"display_name": map[string]interface{}{
					"id":    "display_name",
					"alias": "my_display_name",
				},
				"hostname": map[string]interface{}{
					"id":    "hostname",
					"alias": "foobar",
				},
				"hostname_short": map[string]interface{}{
					"id":    "hostname_short",
					"alias": "foo",
				},
			},
		},
	})
}
