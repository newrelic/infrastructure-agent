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
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	fixture "github.com/newrelic/infrastructure-agent/test/fixture/inventory"
	"github.com/newrelic/infrastructure-agent/test/infra"
	ihttp "github.com/newrelic/infrastructure-agent/test/infra/http"
	"github.com/stretchr/testify/assert"
)

func TestSelinuxRootAccess(t *testing.T) {
	t.Skip("untestable plugin, refactor required")

	if os.Getuid() != 0 {
		t.Skip("Test can only be run as root")
	}

	testClient := ihttp.NewRequestRecorderClient()
	a := infra.NewAgent(testClient.Client)

	// Ge give some room to process the reaps from the three different sources
	conf := a.Context.Config()
	conf.SendInterval = 200 * time.Millisecond
	conf.SelinuxEnableSemodule = true

	a.RegisterPlugin(pluginsLinux.NewSELinuxPlugin(ids.PluginID{"config", "selinux"}, a.Context))

	go a.Run()

	var req http.Request
	select {
	case req = <-testClient.RequestCh:
		a.Terminate()
	case <-time.After(testhelpers.InventoryDuration(conf.SelinuxIntervalSec)):
		a.Terminate()
		assert.FailNow(t, "timeout while waiting for a response")
	}

	fixture.AssertRequestContainsInventoryDeltas(t, req, []*inventoryapi.RawDelta{
		{
			Source:   "config/selinux",
			ID:       1,
			FullDiff: true,
			Diff: map[string]interface{}{
				// Common values for a default installation
				"CurrentMode": map[string]interface{}{
					"id":    "CurrentMode",
					"value": "enforcing",
				},
				"FSMount": map[string]interface{}{
					"id":    "FSMount",
					"value": "/sys/fs/selinux",
				},
				"Status": map[string]interface{}{
					"id":    "Status",
					"value": "enabled",
				},
			},
		},
		{
			Source:   "config/selinux-modules",
			ID:       1,
			FullDiff: true,
			Diff: map[string]interface{}{
				// Looking for some common modules that should be available by
				// default in any SELInux installation
				"ssh": map[string]interface{}{
					"id":      "ssh",
					"version": fixture.AnyValue,
				},
				"su": map[string]interface{}{
					"id":      "su",
					"version": fixture.AnyValue,
				},
				"mandb": map[string]interface{}{
					"id":      "mandb",
					"version": fixture.AnyValue,
				},
				"iptables": map[string]interface{}{
					"id":      "iptables",
					"version": fixture.AnyValue,
				},
				"hostname": map[string]interface{}{
					"id":      "hostname",
					"version": fixture.AnyValue,
				},
				"cron": map[string]interface{}{
					"id":      "cron",
					"version": fixture.AnyValue,
				},
			},
		},
		{
			Source:   "config/selinux-policies",
			ID:       1,
			FullDiff: true,
			Diff: map[string]interface{}{
				"selinuxuser_direct_dri_enabled": map[string]interface{}{
					"id":    "selinuxuser_direct_dri_enabled",
					"value": fixture.AnyValue,
				},
				"selinuxuser_execheap": map[string]interface{}{
					"id":    "selinuxuser_execheap",
					"value": fixture.AnyValue,
				},
				"selinuxuser_ping": map[string]interface{}{
					"id":    "selinuxuser_ping",
					"value": fixture.AnyValue,
				},
				"ssh_keysign": map[string]interface{}{
					"id":    "ssh_keysign",
					"value": fixture.AnyValue,
				},
				"user_exec_content": map[string]interface{}{
					"id":    "user_exec_content",
					"value": fixture.AnyValue,
				},
			},
		},
	})
}
