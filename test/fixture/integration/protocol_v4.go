// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package integration

import (
	"encoding/json"

	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
)

// ProtocolParsingPair payload and corresponding integration protocol parsed struct output.
type ProtocolParsingPair struct {
	Payload  []byte
	ParsedV4 protocol.DataV4
}

var (
	ProtocolV4 = ProtocolParsingPair{
		Payload: []byte(`
	{
	  "protocol_version": "4",
	  "integration": {
		"name": "integration name",
		"version": "integration version"
	  },
	  "data": [
		{
		  "common": {
			"timestamp": 1531414060739,
			"interval.ms": 10000,
			"attributes": {}
		  },
		  "metrics":[
			{
			  "name": "redis.metric1",
			  "type": "count",
			  "value": 93,
			  "attributes": {}
			}
		  ],
		  "entity":{
			"name": "unique name",
			"type": "RedisInstance",
			"displayName": "human readable name",
			"metadata": {}
		  },
		  "inventory": {
			"inventory_foo": {
			  "value": "bar"
			}
		  },
		  "events":[]
		}
	  ]
	}`),
		ParsedV4: protocol.DataV4{
			PluginProtocolVersion: protocol.PluginProtocolVersion{
				RawProtocolVersion: "4",
			},
			Integration: protocol.IntegrationMetadata{
				Name:    "integration name",
				Version: "integration version",
			},
			DataSets: []protocol.Dataset{
				{
					Common: protocol.Common{
						Timestamp:  &ts,
						Interval:   &interval,
						Attributes: map[string]interface{}{}},
					Metrics: []protocol.Metric{
						{
							Name: "redis.metric1",
							Type: "count",
							//Timestamp:  (*int64)(nil),
							//Interval:   (*int64)(nil),
							Attributes: map[string]interface{}{},
							Value:      json.RawMessage("93"),
						},
					},
					Entity: protocol.Entity{
						Name:        "unique name",
						Type:        "RedisInstance",
						DisplayName: "human readable name",
						Metadata:    map[string]interface{}{}},
					Inventory: map[string]protocol.InventoryData{
						"inventory_foo": {"value": "bar"},
					},
					Events: []protocol.EventData{},
				},
			},
		},
	}
	// internal
	interval = int64(10000)
	ts       = int64(1531414060739)
)
