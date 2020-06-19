// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux
// +build harvest

package harvest

import (
	"net/http"
	"os"
	"testing"
	"time"

	pluginsLinux "github.com/newrelic/infrastructure-agent/internal/plugins/linux"
	"github.com/newrelic/infrastructure-agent/internal/testhelpers"
	"github.com/newrelic/infrastructure-agent/pkg/backend/inventoryapi"
	fixture "github.com/newrelic/infrastructure-agent/test/fixture/inventory"
	"github.com/newrelic/infrastructure-agent/test/infra"
	ihttp "github.com/newrelic/infrastructure-agent/test/infra/http"
	"github.com/stretchr/testify/assert"
)

func TestRpmPlugin(t *testing.T) {
	if _, err := os.Stat(pluginsLinux.RpmPath); os.IsNotExist(err) {
		t.Skip("This test must be executed in RPM-based distributions")
	}

	testhelpers.SetupLog()

	testClient := ihttp.NewRequestRecorderClient()
	a := infra.NewAgent(testClient.Client)
	a.RegisterPlugin(pluginsLinux.NewRpmPlugin(a.Context))

	go a.Run()

	var req http.Request
	select {
	case req = <-testClient.RequestCh:
		a.Terminate()
	case <-time.After(testhelpers.InventoryDuration(a.Context.Config().RpmRefreshSec)):
		a.Terminate()
		assert.FailNow(t, "timeout while waiting for a response")
	}

	// Verify common, usual packages that should be available in any distribution
	fixture.AssertRequestContainsInventoryDeltas(t, req, []*inventoryapi.RawDelta{
		{
			Source:   "packages/rpm",
			ID:       1,
			FullDiff: true,
			Diff: map[string]interface{}{
				// Common value for a default installation
				"rpm": map[string]interface{}{
					"architecture":    "x86_64",
					"id":              "rpm",
					"installed_epoch": fixture.AnyValue,
					"release":         fixture.AnyValue,
					"version":         fixture.AnyValue,
				},
			},
		},
	})

}
