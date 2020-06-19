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
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	fixture "github.com/newrelic/infrastructure-agent/test/fixture/inventory"
	"github.com/newrelic/infrastructure-agent/test/infra"
	ihttp "github.com/newrelic/infrastructure-agent/test/infra/http"
	"github.com/stretchr/testify/assert"
)

func TestSysctlPollingRootAccess(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Test can only be run as root")
	}

	const timeout = 5 * time.Second

	testClient := ihttp.NewRequestRecorderClient()
	a := infra.NewAgent(testClient.Client)

	a.RegisterPlugin(pluginsLinux.NewSysctlPollingMonitor(ids.PluginID{"kernel", "sysctl"}, a.Context))

	go a.Run()

	var req http.Request
	select {
	case req = <-testClient.RequestCh:
		a.Terminate()
	case <-time.After(timeout):
		a.Terminate()
		assert.FailNow(t, "timeout while waiting for a response")
	}

	fixture.AssertRequestContainsInventoryDeltas(t, req, fixture.ExpectedSysctlDelta)
}
