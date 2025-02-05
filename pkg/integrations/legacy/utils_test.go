// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package legacy

import (
	"testing"

	"github.com/newrelic/infrastructure-agent/internal/agent/types"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
	"github.com/stretchr/testify/assert"
)

func TestBuildInventoryDataSet(t *testing.T) {
	elog := rlog.WithField("action", "EmitDataSet")

	tests := []struct {
		name            string
		inventoryData   map[string]protocol.InventoryData
		labels          map[string]string
		customAttr      map[string]string
		integrationUser string
		pluginName      string
		pluginVersion   string
		reportingAgent  string
		entityKey       string
		want            int
		wantLabelValues map[string]string
	}{
		{
			name:            "empty input data",
			inventoryData:   map[string]protocol.InventoryData{},
			labels:          map[string]string{},
			customAttr:      map[string]string{},
			entityKey:       "entity1",
			want:            0,
			wantLabelValues: map[string]string{},
		},
		{
			name: "basic inventory data",
			inventoryData: map[string]protocol.InventoryData{
				"test1": {"name": "item1", "value": "value1"},
				"test2": {"name": "item2", "value": "value2"},
			},
			labels:          map[string]string{},
			customAttr:      map[string]string{},
			entityKey:       "entity1",
			want:            2,
			wantLabelValues: map[string]string{},
		},
		{
			name:          "with labels",
			inventoryData: map[string]protocol.InventoryData{},
			labels: map[string]string{
				"env":    "prod",
				"region": "us-east",
			},
			customAttr: map[string]string{},
			entityKey:  "entity1",
			want:       2,
			wantLabelValues: map[string]string{
				"labels/env":    "prod",
				"labels/region": "us-east",
			},
		},
		{
			name:          "with custom attributes",
			inventoryData: map[string]protocol.InventoryData{},
			labels:        map[string]string{},
			customAttr: map[string]string{
				"team":  "ohai",
				"owner": "david",
			},
			entityKey: "entity1",
			want:      2,
			wantLabelValues: map[string]string{
				"labels/team":  "ohai",
				"labels/owner": "david",
			},
		},
		{
			name:          "custom attributes with duplicate labels",
			inventoryData: map[string]protocol.InventoryData{},
			labels: map[string]string{
				"team": "infra",
			},
			customAttr: map[string]string{
				"team":  "ohai",
				"owner": "david",
			},
			entityKey: "entity1",
			want:      2,
			wantLabelValues: map[string]string{
				"labels/team":  "infra",
				"labels/owner": "david",
			},
		},
		{
			name:          "verify labels precedence over custom attributes",
			inventoryData: map[string]protocol.InventoryData{},
			labels: map[string]string{
				"team": "infra",
				"env":  "production",
			},
			customAttr: map[string]string{
				"team": "ohai",
				"env":  "staging",
			},
			entityKey: "entity1",
			want:      2,
			wantLabelValues: map[string]string{
				"labels/team": "infra",
				"labels/env":  "production",
			},
		},
		{
			name: "with all integration metadata",
			inventoryData: map[string]protocol.InventoryData{
				"test1": {"name": "item1", "value": "value1"},
			},
			labels:          map[string]string{"env": "prod"},
			customAttr:      map[string]string{"owner": "john"},
			integrationUser: "serviceAccount",
			pluginName:      "test-plugin",
			pluginVersion:   "1.0.0",
			reportingAgent:  "agent1",
			entityKey:       "entity1",
			want:            7,
			wantLabelValues: map[string]string{
				"labels/env":         "prod",
				"labels/owner":       "john",
				integrationUserID:    "serviceAccount",
				integrationNameID:    "test-plugin",
				integrationVersionID: "1.0.0",
				reportingAgentID:     "agent1",
			},
		},
		{
			name:            "partial integration metadata",
			inventoryData:   map[string]protocol.InventoryData{},
			labels:          map[string]string{},
			customAttr:      map[string]string{},
			integrationUser: "serviceAccount",
			pluginName:      "test-plugin",
			entityKey:       "entity1",
			want:            2,
			wantLabelValues: map[string]string{
				integrationUserID: "serviceAccount",
				integrationNameID: "test-plugin",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildInventoryDataSet(
				elog,
				tt.inventoryData,
				tt.labels,
				tt.customAttr,
				tt.integrationUser,
				tt.pluginName,
				tt.pluginVersion,
				tt.reportingAgent,
				tt.entityKey,
			)

			assert.Len(t, result, tt.want, "unexpected number of items in result")

			// Check for expected labels and their values
			for labelID, expectedValue := range tt.wantLabelValues {
				exists, actualValue := findItemAndValueInPluginInventoryDataset(result, labelID)
				assert.Truef(t, exists, "missing expected label %s", labelID)
				assert.Equal(t, expectedValue, actualValue, "unexpected value for label %s", labelID)
			}

			// Verify inventory data items exist
			for key := range tt.inventoryData {
				exists, _ := findItemAndValueInPluginInventoryDataset(result, key)
				assert.Truef(t, exists, "missing inventory item with ID %s", key)
			}
		})
	}
}

func findItemAndValueInPluginInventoryDataset(items types.PluginInventoryDataset, id string) (bool, string) {
	for _, item := range items {
		if item.SortKey() == id {
			if invData, ok := item.(protocol.InventoryData); ok {
				if value, ok := invData["value"].(string); ok {
					return true, value
				}
			}
			return true, ""
		}
	}
	return false, ""
}
