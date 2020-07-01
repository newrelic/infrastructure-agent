// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux
// +build harvest

package harvest

import (
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/backend/inventoryapi"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/plugins"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	fixture "github.com/newrelic/infrastructure-agent/test/fixture/inventory"
	"github.com/newrelic/infrastructure-agent/test/infra"
	ihttp "github.com/newrelic/infrastructure-agent/test/infra/http"
	"github.com/stretchr/testify/assert"
)

func TestPluginCustomAttributes(t *testing.T) {
	const timeout = 5 * time.Second

	testClient := ihttp.NewRequestRecorderClient()
	a := infra.NewAgent(testClient.Client, func(cfg *config.Config) {
		cfg.CustomAttributes = config.CustomAttributeMap{
			"custom_attribute_foo": "bar",
		}
	})
	a.RegisterPlugin(plugins.NewCustomAttrsPlugin(a.Context))

	go a.Run()
	defer a.Terminate()

	select {
	case req := <-testClient.RequestCh:
		pluginID := ids.CustomAttrsID
		attrs := plugins.CustomAttrs{}
		fixture.AssertRequestContainsInventoryDeltas(t, req, []*inventoryapi.RawDelta{
			{
				ID:       1,
				Source:   pluginID.String(),
				FullDiff: true,
				Diff: map[string]interface{}{
					attrs.SortKey(): a.Context.Config().CustomAttributes,
				},
			},
		})
	case <-time.After(timeout):
		assert.FailNow(t, "timeout while waiting for a response")
	}
}
