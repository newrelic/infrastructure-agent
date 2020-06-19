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
	"github.com/newrelic/infrastructure-agent/pkg/backend/inventoryapi"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	fixture "github.com/newrelic/infrastructure-agent/test/fixture/inventory"
	"github.com/newrelic/infrastructure-agent/test/infra"
	ihttp "github.com/newrelic/infrastructure-agent/test/infra/http"
	"github.com/stretchr/testify/assert"
)

// The difference between this test and the one in the `parent` folder, is that this
// is triggered against a real /etc/ssh/sshd_config file to check the required privileges
// are set. This test will fail if not run as a root or a privileged user
func TestSshdConfigRootAccess(t *testing.T) {
	t.Skip("untestable plugin, refactor required")

	if os.Getuid() != 0 {
		t.Skip("Test can only be run as root")
	}

	const timeout = 5 * time.Second

	testClient := ihttp.NewRequestRecorderClient()
	a := infra.NewAgent(testClient.Client)

	a.RegisterPlugin(pluginsLinux.NewSshdConfigPlugin(ids.PluginID{"config", "sshd"}, a.Context))

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
			Source:   "config/sshd",
			ID:       1,
			FullDiff: true,
			// Checking some common kernel modules that should exist in any linux host
			Diff: map[string]interface{}{
				"PasswordAuthentication": fixture.AnyValue,
			},
		},
	})
}
