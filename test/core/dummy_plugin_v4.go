// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"testing"

	"github.com/newrelic/infrastructure-agent/internal/agent/types"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/feature_flags/test"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/fixtures"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/testhelp"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/fwrequest"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/legacy"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/config"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/dm"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/stretchr/testify/require"
)

type dummyV4Plugin struct {
	agent.PluginCommon
	ticker  chan interface{}
	payload []byte
	t       *testing.T
}

func newDummyV4Plugin(t *testing.T, payload string, context agent.AgentContext) *dummyV4Plugin {
	return &dummyV4Plugin{
		PluginCommon: agent.PluginCommon{
			ID:      ids.PluginID{"test", "dummy_v4"},
			Context: context,
		},
		ticker:  make(chan interface{}),
		payload: []byte(payload),
		t:       t,
	}
}

func (cp *dummyV4Plugin) Run() {
	for {
		select {
		case <-cp.ticker:
			dss := InventoryDatasetsForPayload(cp.t, cp.payload)
			for _, ds := range dss {
				cp.EmitInventory(ds, entity.NewFromNameWithoutID(cp.Context.EntityKey()))
			}
		}
	}
}

func (cp *dummyV4Plugin) Id() ids.PluginID {
	return cp.ID
}

func (cp *dummyV4Plugin) harvest() {
	cp.ticker <- 1
}

func InventoryDatasetsForPayload(t *testing.T, payload []byte) (dss []types.PluginInventoryDataset) {
	dataV4, err := dm.ParsePayloadV4(payload, test.NewFFRetrieverReturning(true, true))
	require.NoError(t, err)

	def, err := integration.NewDefinition(config.ConfigEntry{
		InstanceName: "foo",
		Exec:         testhelp.Command(fixtures.BasicCmd),
	}, integration.ErrLookup, nil, nil)
	require.NoError(t, err)

	r := fwrequest.NewFwRequest(def, nil, nil, dataV4)
	for _, ds := range r.Data.DataSets {

		legacyDS := legacy.BuildInventoryDataSet(
			log.WithComponent("test"),
			ds.Inventory,
			nil,
			"integrationUser",
			dataV4.Integration.Name,
			ds.Entity.Name,
		)

		dss = append(dss, legacyDS)
	}

	return
}
