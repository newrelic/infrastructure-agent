// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux
// +build harvest

package harvest

import (
	"github.com/newrelic/infrastructure-agent/pkg/entity"
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

func TestSystemdPlugin(t *testing.T) {
	t.Skip("untestable plugin, refactor required")

	if _, err := os.Stat("/bin/systemctl"); os.IsNotExist(err) {
		t.Skip("This test must be executed in a SystemD supporting OS")
	}

	testClient := ihttp.NewRequestRecorderClient()
	a := infra.NewAgent(testClient.Client)
	a.Context.SetAgentIdentity(entity.Identity{10, "abcdef"})
	a.RegisterPlugin(pluginsLinux.NewSystemdPlugin(a.Context))

	go a.Run()

	var req http.Request
	select {
	case req = <-testClient.RequestCh:
		a.Terminate()
	case <-time.After(testhelpers.InventoryDuration(a.Context.Config().SystemdIntervalSec)):
		a.Terminate()
		assert.FailNow(t, "timeout while waiting for a response")
	}

	fixture.AssertRequestContainsInventoryDeltas(t, req, []*inventoryapi.RawDelta{
		{
			Source:   "services/systemd",
			ID:       1,
			FullDiff: true,
			// Checking some common services that should exist in any linux host
			Diff: map[string]interface{}{
				"NetworkManager": map[string]interface{}{
					"id":  "NetworkManager",
					"pid": fixture.AnyValue,
				},
				"sshd": map[string]interface{}{
					"id":  "sshd",
					"pid": fixture.AnyValue,
				},
			},
		},
	})
}
