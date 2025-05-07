// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux && harvest
// +build linux,harvest

package harvest

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/entity"

	"github.com/fsnotify/fsnotify"
	pluginsLinux "github.com/newrelic/infrastructure-agent/internal/plugins/linux"
	"github.com/newrelic/infrastructure-agent/internal/testhelpers"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	fixture "github.com/newrelic/infrastructure-agent/test/fixture/inventory"
	"github.com/newrelic/infrastructure-agent/test/infra"
	ihttp "github.com/newrelic/infrastructure-agent/test/infra/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSysctlPollingRootless(t *testing.T) {
	testClient := ihttp.NewRequestRecorderClient()
	a := infra.NewAgent(testClient.Client)
	a.Context.Config().SysctlIntervalSec = 10 // fail faster

	a.RegisterPlugin(pluginsLinux.NewSysctlPollingMonitor(ids.PluginID{"kernel", "sysctl"}, a.Context))
	a.Context.SetAgentIdentity(entity.Identity{10, "abcdef"})
	go a.Run()

	var req http.Request
	timeout := testhelpers.InventoryDuration(a.Context.Config().SysctlIntervalSec)
	select {
	case req = <-testClient.RequestCh:
		a.Terminate()
	case <-time.After(timeout):
		a.Terminate()
		assert.FailNow(t, "timeout while waiting for a response", timeout.String())
	}

	fixture.AssertRequestContainsInventoryDeltas(t, req, fixture.ExpectedSysctlDelta)
}

func TestSysctlSubscriberRootless(t *testing.T) {
	testClient := ihttp.NewRequestRecorderClient()
	a := infra.NewAgent(testClient.Client)
	a.Context.Config().SysctlIntervalSec = 10 // fail faster
	a.Context.SetAgentIdentity(entity.Identity{10, "abcdef"})

	p, err := pluginsLinux.NewSysctlSubscriberMonitor(ids.PluginID{"kernel", "sysctl"}, a.Context)
	require.NoError(t, err)
	a.RegisterPlugin(p)

	go a.Run()

	var req http.Request
	timeout := testhelpers.InventoryDuration(a.Context.Config().SysctlIntervalSec)
	select {
	case req = <-testClient.RequestCh:
		a.Terminate()
	case <-time.After(timeout):
		a.Terminate()
		assert.FailNow(t, "timeout while waiting for a response", timeout.String())
	}

	fixture.AssertRequestContainsInventoryDeltas(t, req, fixture.ExpectedSysctlDelta)
}

func BenchmarkSysctlPollingRootless(b *testing.B) {
	testClient := ihttp.NewRequestRecorderClient()
	a := infra.NewAgent(testClient.Client)
	a.Context.SetAgentIdentity(entity.Identity{10, "abcdef"})

	// avoid event submission channel being blocked by no consumers
	go a.Run()

	p := pluginsLinux.NewSysctlPollingMonitor(ids.PluginID{"kernel", "sysctl"}, a.Context)

	b.StartTimer()

	for i := 0; i < b.N; i++ {
		_, err := p.Sysctls()
		if err != nil {
			b.Error(err.Error())
		}
	}
}

func BenchmarkSysctlSubscriberRootless(b *testing.B) {
	testClient := ihttp.NewRequestRecorderClient()
	a := infra.NewAgent(testClient.Client)
	a.Context.SetAgentIdentity(entity.Identity{10, "abcdef"})

	// avoid event submission channel being blocked by no consumers
	go a.Run()

	p, err := pluginsLinux.NewSysctlSubscriberMonitor(ids.PluginID{"kernel", "sysctl"}, a.Context)
	if err != nil {
		b.Fatalf("%v", err.Error())
		return
	}

	b.StartTimer()

	evCh := p.EventsCh()
	go p.Run()

	// fake some FS changes
	for i := 0; i < b.N; i++ {
		evCh <- fsnotify.Event{
			Name: fmt.Sprintf("fake/path/iteration-%d", i),
		}
	}
}
