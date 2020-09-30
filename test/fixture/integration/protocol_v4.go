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

// aims to avoid data mutation when processing structs from global fixture variables
func (p *ProtocolParsingPair) Clone() (clone ProtocolParsingPair) {
	clone = *p
	dss := clone.ParsedV4.DataSets
	clone.ParsedV4.DataSets = []protocol.Dataset{}
	for _, ds := range dss {

		// Common.Attributes
		ca := make(map[string]interface{}, len(ds.Common.Attributes))
		for k, v := range ds.Common.Attributes {
			ca[k] = v
		}
		ds.Common.Attributes = ca

		// Metrics.Attributes
		ms := make([]protocol.Metric, len(ds.Metrics))
		for _, m := range ds.Metrics {
			a := make(map[string]interface{}, len(m.Attributes))
			for k, v := range m.Attributes {
				a[k] = v
			}
			m.Attributes = a
		}
		ds.Metrics = ms

		// same might be required for Events & Inventory

		clone.ParsedV4.DataSets = append(clone.ParsedV4.DataSets, ds)
	}
	return clone
}

var (
	ProtocolV4TwoEntities = ProtocolParsingPair{
		Payload: []byte(`{
  "protocol_version": "4",
  "integration": {
    "name": "Sample",
    "version": "1.2.3"
  },
  "data": [
    {
      "common": {
        "timestamp": 1586357933,
        "interval.ms": 10000,
        "attributes": {
          "host.name": "host-test",
          "host.user": "test-user"
        }
      },
      "metrics": [
        {
          "name": "a.gauge",
          "type": "gauge",
          "value": 13,
          "attributes": {
            "key1": "val1"
          }
        },
        {
          "name": "a.summary",
          "type": "summary",
          "value": {
            "count": 10,
            "sum": 664,
            "min": 15,
            "max": 248
          }
        },
        {
          "name": "a.count",
          "type": "count",
          "value": 666
        }
      ],
      "entity": {
        "name": "a.entity.one",
        "type": "ATYPE",
        "displayName": "A display name one",
        "metadata": {
          "env": "testing"
        }
      },
      "inventory": {
        "inventory_payload_one": {
          "value": "foo-one"
        }
      },
      "events": []
    },
    {
      "common": {
        "timestamp": 1586357933,
        "interval.ms": 10000,
        "attributes": {
          "host.name": "host-test",
          "host.user": "test-user"
        }
      },
      "metrics": [
        {
          "name": "b.gauge",
          "type": "gauge",
          "value": 13,
          "attributes": {
            "key1": "val2"
          }
        },
        {
          "name": "b.summary",
          "type": "summary",
          "value": {
            "count": 10,
            "sum": 664,
            "min": 15,
            "max": 248
          }
        },
        {
          "name": "b.count",
          "type": "count",
          "value": 666
        }
      ],
      "entity": {
        "name": "b.entity.two",
        "type": "ATYPE",
        "displayName": "A display name two",
        "metadata": {
          "env": "testing"
        }
      },
      "inventory": {
        "inventory_payload_two": {
          "value": "bar-two"
        }
      },
      "events": []
    }
  ]
}`),
		ParsedV4: protocol.DataV4{
			PluginProtocolVersion: protocol.PluginProtocolVersion{
				RawProtocolVersion: "4",
			},
			Integration: protocol.IntegrationMetadata{
				Name:    "Sample",
				Version: "1.2.3",
			},
			DataSets: []protocol.Dataset{
				{
					Common: protocol.Common{
						Timestamp:  &ts,
						Interval:   &interval,
						Attributes: map[string]interface{}{}},
					Metrics: []protocol.Metric{
						{
							Name: "b.gauge",
							Type: "gauge",
							//Timestamp:  (*int64)(nil),
							//Interval:   (*int64)(nil),
							Attributes: map[string]interface{}{
								"key1": "val2",
							},
							Value: json.RawMessage("13"),
						},
					},
					Entity: protocol.Entity{
						Name:        "a.entity.one",
						Type:        "ATYPE",
						DisplayName: "A display name one",
						Metadata: map[string]interface{}{
							"env": "testing",
						},
					},
					Inventory: map[string]protocol.InventoryData{
						"inventory_payload_two": {"value": "bar-two"},
					},
					Events: []protocol.EventData{},
				},
				{
					Common: protocol.Common{
						Timestamp:  &ts,
						Interval:   &interval,
						Attributes: map[string]interface{}{}},
					Metrics: []protocol.Metric{
						{
							Name: "a.gauge",
							Type: "gauge",
							//Timestamp:  (*int64)(nil),
							//Interval:   (*int64)(nil),
							Attributes: map[string]interface{}{
								"key1": "val1",
							},
							Value: json.RawMessage("13"),
						},
					},
					Entity: protocol.Entity{
						Name:        "b.entity.two",
						Type:        "ATYPE",
						DisplayName: "A display name two",
						Metadata: map[string]interface{}{
							"env": "testing",
						},
					},
					Inventory: map[string]protocol.InventoryData{
						"inventory_payload_two": {"value": "bar-two"},
					},
					Events: []protocol.EventData{},
				},
			},
		},
	}

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
