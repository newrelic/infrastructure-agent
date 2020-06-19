// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux
// +build harvest

package harvest

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	pluginsLinux "github.com/newrelic/infrastructure-agent/internal/plugins/linux"
	"github.com/newrelic/infrastructure-agent/internal/testhelpers"
	"github.com/newrelic/infrastructure-agent/pkg/backend/inventoryapi"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/newrelic/infrastructure-agent/test/infra"
	ihttp "github.com/newrelic/infrastructure-agent/test/infra/http"
	"github.com/stretchr/testify/assert"
)

func TestKernelModules(t *testing.T) {
	t.Skip()

	testClient := ihttp.NewRequestRecorderClient()
	a := infra.NewAgent(testClient.Client)

	a.RegisterPlugin(pluginsLinux.NewKernelModulesPlugin(ids.PluginID{"kernel", "modules"}, a.Context))

	go a.Run()

	var req http.Request
	select {
	case req = <-testClient.RequestCh:
		a.Terminate()
	case <-time.After(testhelpers.InventoryDuration(a.Context.Config().KernelModulesRefreshSec)):
		a.Terminate()
		assert.FailNow(t, "timeout while waiting for a response")
	}

	reqBytes, err := ioutil.ReadAll(req.Body)
	assert.NoError(t, err)
	sent := inventoryapi.PostDeltaBody{}
	assert.NoError(t, json.Unmarshal(reqBytes, &sent))

	delta := sent.Deltas[0]
	assert.Equal(t, "kernel/modules", delta.Source)
	assert.EqualValues(t, 1, delta.ID)
	assert.True(t, delta.FullDiff)

	// checking that at least one of the following modules are reported (may vary depending on
	// different distros)
	modules := map[string]struct{}{
		"glue_helper": {}, "lrw": {}, "ip_tables": {}, "isofs": {}, "serio_raw": {}, "ipv6": {},
		"psmouse": {}, "dm_mod": {}, "ttm": {},
	}

	for module, diff := range delta.Diff {
		if _, ok := modules[module]; ok {
			if d, ok := diff.(map[string]interface{}); !ok {
				assert.FailNow(t, "expecting map for diff", "module: %v. got: %#v", module, diff)
			} else {
				assert.Equal(t, module, d["id"])
				assert.Contains(t, d, "version")
				assert.Contains(t, d, "description")
			}
			return
		}
	}

	assert.FailNow(t, "can't find any module", "looking for %v. Found %v", modules, delta.Diff)
}
