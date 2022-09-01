// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package integration

import (
	"encoding/json"

	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
)

// ProtocolParsingPair payload and corresponding integration protocol parsed struct output.
type ProtocolParsingPair struct {
	Payload  []byte
	ParsedV4 protocol.DataV4
}

var (
	// nolint:gochecknoglobals
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
        "name": "entity.name",
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
        "name": "entity.name",
        "type": "BTYPE",
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
      "events":[
		{ 
		  "summary": "foo",
		  "format": "event",
		  "attributes": { "format": "attribute"}
		}
	  ]
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
						Attributes: map[string]interface{}{},
					},
					Metrics: []protocol.Metric{
						{
							Name: "b.gauge",
							Type: "gauge",
							// Timestamp:  (*int64)(nil),
							// Interval:   (*int64)(nil),
							Attributes: map[string]interface{}{
								"key1": "val2",
							},
							Value: json.RawMessage("13"),
						},
					},
					Entity: entity.Fields{
						Name:        "entity.name",
						Type:        "ATYPE",
						DisplayName: "A display name one",
						Metadata: map[string]interface{}{
							"env": "testing",
						},
					},
					Inventory: map[string]protocol.InventoryData{
						"inventory_payload_one": {"value": "foo-one"},
					},
					Events: []protocol.EventData{},
				},
				{
					Common: protocol.Common{
						Timestamp:  &ts,
						Interval:   &interval,
						Attributes: map[string]interface{}{},
					},
					Metrics: []protocol.Metric{
						{
							Name: "a.gauge",
							Type: "gauge",
							// Timestamp:  (*int64)(nil),
							// Interval:   (*int64)(nil),
							Attributes: map[string]interface{}{
								"key1": "val1",
							},
							Value: json.RawMessage("13"),
						},
					},
					Entity: entity.Fields{
						Name:        "entity.name",
						Type:        "BTYPE",
						DisplayName: "A display name two",
						Metadata: map[string]interface{}{
							"env": "testing",
						},
					},
					Inventory: map[string]protocol.InventoryData{
						"inventory_payload_two": {"value": "bar-two"},
					},
					Events: []protocol.EventData{
						{
							"summary": "foo",
							"format":  "event",
							"attributes": map[string]interface{}{
								"format": "attribute",
							},
						},
					},
				},
			},
		},
	}

	// nolint:gochecknoglobals
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
		  "ignore_entity": false,
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
		  "events":[
				{ 
				  "summary": "foo",
				  "format": "event",
				  "attributes": { "format": "attribute"}
				}
		  ]
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
					IgnoreEntity: false,
					Common: protocol.Common{
						Timestamp:  &ts,
						Interval:   &interval,
						Attributes: map[string]interface{}{}},
					Metrics: []protocol.Metric{
						{
							Name: "redis.metric1",
							Type: "count",
							// Timestamp:  (*int64)(nil),
							// Interval:   (*int64)(nil),
							Attributes: map[string]interface{}{},
							Value:      json.RawMessage("93"),
						},
					},
					Entity: entity.Fields{
						Name:        "unique name",
						Type:        "RedisInstance",
						DisplayName: "human readable name",
						Metadata:    map[string]interface{}{},
					},
					Inventory: map[string]protocol.InventoryData{
						"inventory_foo": {"value": "bar"},
					},
					Events: []protocol.EventData{
						{
							"summary": "foo",
							"format":  "event",
							"attributes": map[string]interface{}{
								"format": "attribute",
							},
						},
					},
				},
			},
		},
	}

	// nolint:gochecknoglobals
	ProtocolV4IgnoreEntity = ProtocolParsingPair{
		Payload: []byte(`
	{
	  "protocol_version": "4",
	  "integration": {
		"name": "integration name",
		"version": "integration version"
	  },
	  "data": [
		{
		  "ignore_entity": true,
		  "common": {
			"timestamp": 1531414060739,
			"interval.ms": 10000,
			"attributes": {
			  "targetName": "localhost:9178",
			  "scrapeUrl": "http://localhost:9178",
			}
		  },
		  "metrics":[
			{
			  "name": "redis.metric1",
			  "type": "count",
			  "value": 93,
			  "attributes": {}
			},
			{
			  "name": "redis.metric2",
			  "type": "count",
			  "value": 94,
			  "attributes": {}
			}
		  ],
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
					IgnoreEntity: true,
					Common: protocol.Common{
						Timestamp: &ts,
						Interval:  &interval,
						Attributes: map[string]interface{}{
							"targetName": "localhost:9178",
							"scrapeUrl":  "http://localhost:9178",
						},
					},
					Metrics: []protocol.Metric{
						{
							Name: "redis.metric1",
							Type: "count",
							// Timestamp:  (*int64)(nil),
							// Interval:   (*int64)(nil),
							Attributes: map[string]interface{}{},
							Value:      json.RawMessage("93"),
						},
						{
							Name: "redis.metric2",
							Type: "count",
							// Timestamp:  (*int64)(nil),
							// Interval:   (*int64)(nil),
							Attributes: map[string]interface{}{},
							Value:      json.RawMessage("94"),
						},
					},
				},
			},
		},
	}

	// nolint:gochecknoglobals
	ProtocolV4DontIgnoreEntityIntegration = ProtocolParsingPair{
		Payload: []byte(`
	{
	  "protocol_version": "4",
	  "integration": {
		"name": "integration name",
		"version": "integration version"
	  },
	  "data": [
		{
		  "ignore_entity": false,
		  "common": {},
		  "entity": {
			"name": "WIN_SERVICE:localhost:w32time",
			"displayName": "Windows Time",
			"type": "WIN_SERVICE",
			"metadata": {
			  "display_name": "Windows Time",
			  "hostname": "EC2AMAZ-CTQEKQM",
			  "process_id": "1784",
			  "run_as": "NT AUTHORITY\\LocalService",
			  "service_name": "w32time",
			  "start_mode": "auto"
			}
		  },
		  "metrics":[
			{
			  "name": "redis.metric1",
			  "type": "count",
			  "value": 93,
			  "attributes": {}
			},
			{
			  "name": "redis.metric2",
			  "type": "count",
			  "value": 94,
			  "attributes": {}
			}
		  ],
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
					IgnoreEntity: false,
					Entity: entity.Fields{
						Name:        "WIN_SERVICE:localhost:w32time",
						Type:        "WIN_SERVICE",
						DisplayName: "Windows Time",
						Metadata: map[string]interface{}{
							"display_name": "Windows Time",
							"hostname":     "EC2AMAZ-CTQEKQM",
							"process_id":   "1784",
							"run_as":       "NT AUTHORITY\\LocalService",
							"service_name": "w32time",
							"start_mode":   "auto",
						},
					},
					Common: protocol.Common{
						Timestamp: &ts,
						Interval:  &interval,
						Attributes: map[string]interface{}{
							"targetName": "localhost:9178",
							"scrapeUrl":  "http://localhost:9178",
						},
					},
					Metrics: []protocol.Metric{
						{
							Name: "redis.metric1",
							Type: "count",
							// Timestamp:  (*int64)(nil),
							// Interval:   (*int64)(nil),
							Attributes: map[string]interface{}{},
							Value:      json.RawMessage("93"),
						},
						{
							Name: "redis.metric2",
							Type: "count",
							// Timestamp:  (*int64)(nil),
							// Interval:   (*int64)(nil),
							Attributes: map[string]interface{}{},
							Value:      json.RawMessage("94"),
						},
					},
				},
			},
		},
	}
	// nolint:gochecknoglobals
	ProtocolV4NoEntityDontIgnoreEntityAgent = ProtocolParsingPair{
		Payload: []byte(`
	{
	  "protocol_version": "4",
	  "integration": {
		"name": "integration name",
		"version": "integration version"
	  },
	  "data": [
		{
		  "ignore_entity": false,
		  "common": {
			"timestamp": 1531414060739,
			"interval.ms": 10000,
			"attributes": {
			  "targetName": "localhost:9178",
			  "scrapeUrl": "http://localhost:9178",
			}
		  },
		  "metrics":[
			{
			  "name": "redis.metric1",
			  "type": "count",
			  "value": 93,
			  "attributes": {}
			},
			{
			  "name": "redis.metric2",
			  "type": "count",
			  "value": 94,
			  "attributes": {}
			}
		  ],
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
					IgnoreEntity: false,
					Common: protocol.Common{
						Timestamp: &ts,
						Interval:  &interval,
						Attributes: map[string]interface{}{
							"targetName": "localhost:9178",
							"scrapeUrl":  "http://localhost:9178",
						},
					},
					Metrics: []protocol.Metric{
						{
							Name: "redis.metric1",
							Type: "count",
							// Timestamp:  (*int64)(nil),
							// Interval:   (*int64)(nil),
							Attributes: map[string]interface{}{},
							Value:      json.RawMessage("93"),
						},
						{
							Name: "redis.metric2",
							Type: "count",
							// Timestamp:  (*int64)(nil),
							// Interval:   (*int64)(nil),
							Attributes: map[string]interface{}{},
							Value:      json.RawMessage("94"),
						},
					},
				},
			},
		},
	}
	// internal.
	interval = int64(10000)         // nolint:gochecknoglobals,gomnd
	ts       = int64(1531414060739) // nolint:gochecknoglobals,gomnd
)
