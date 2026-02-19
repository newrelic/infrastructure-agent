// Copyright 2026 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package sample

import (
	"encoding/json"
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBaseEvent_Type(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
	}{
		{
			name:      "standard event type",
			eventType: "SystemSample",
		},
		{
			name:      "custom event type",
			eventType: "CustomEvent",
		},
		{
			name:      "empty event type",
			eventType: "",
		},
		{
			name:      "event type with special characters",
			eventType: "Event_Type-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			be := &BaseEvent{EventType: "", Timestmp: 0, EntityKey: ""}
			be.Type(tt.eventType)
			assert.Equal(t, tt.eventType, be.EventType)
		})
	}
}

func TestBaseEvent_Entity(t *testing.T) {
	tests := []struct {
		name        string
		entityKey   entity.Key
		expectedKey string
	}{
		{
			name:        "standard entity key",
			entityKey:   entity.Key("host:my-hostname"),
			expectedKey: "host:my-hostname",
		},
		{
			name:        "empty entity key",
			entityKey:   entity.Key(""),
			expectedKey: "",
		},
		{
			name:        "entity key with special characters",
			entityKey:   entity.Key("container:abc-123_def"),
			expectedKey: "container:abc-123_def",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			be := &BaseEvent{EventType: "", Timestmp: 0, EntityKey: ""}
			be.Entity(tt.entityKey)
			assert.Equal(t, tt.expectedKey, be.EntityKey)
		})
	}
}

func TestBaseEvent_Timestamp(t *testing.T) {
	tests := []struct {
		name      string
		timestamp int64
	}{
		{
			name:      "positive timestamp",
			timestamp: 1609459200000,
		},
		{
			name:      "zero timestamp",
			timestamp: 0,
		},
		{
			name:      "negative timestamp",
			timestamp: -1000,
		},
		{
			name:      "max int64 timestamp",
			timestamp: 9223372036854775807,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			be := &BaseEvent{EventType: "", Timestmp: 0, EntityKey: ""}
			be.Timestamp(tt.timestamp)
			assert.Equal(t, tt.timestamp, be.Timestmp)
		})
	}
}

func TestBaseEvent_JSONMarshaling(t *testing.T) {
	tests := []struct {
		name     string
		event    BaseEvent
		expected map[string]any
	}{
		{
			name: "full event",
			event: BaseEvent{
				EventType: "SystemSample",
				Timestmp:  1609459200000,
				EntityKey: "host:my-host",
			},
			expected: map[string]any{
				"eventType": "SystemSample",
				"timestamp": float64(1609459200000),
				"entityKey": "host:my-host",
			},
		},
		{
			name:  "empty event",
			event: BaseEvent{EventType: "", Timestmp: 0, EntityKey: ""},
			expected: map[string]any{
				"eventType": "",
				"timestamp": float64(0),
				"entityKey": "",
			},
		},
	}

	for _, tt := range tests { //nolint:varnamelen
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(&tt.event)
			require.NoError(t, err)

			var result map[string]any

			err = json.Unmarshal(data, &result)
			require.NoError(t, err)

			assert.Equal(t, tt.expected["eventType"], result["eventType"])
			assert.Equal(t, tt.expected["timestamp"], result["timestamp"])
			assert.Equal(t, tt.expected["entityKey"], result["entityKey"])
		})
	}
}

func TestBaseEvent_JSONUnmarshaling(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		expected BaseEvent
	}{
		{
			name:     "full event",
			jsonData: `{"eventType":"SystemSample","timestamp":1609459200000,"entityKey":"host:my-host"}`,
			expected: BaseEvent{
				EventType: "SystemSample",
				Timestmp:  1609459200000,
				EntityKey: "host:my-host",
			},
		},
		{
			name:     "empty event",
			jsonData: `{}`,
			expected: BaseEvent{EventType: "", Timestmp: 0, EntityKey: ""},
		},
	}

	for _, tt := range tests { //nolint:varnamelen
		t.Run(tt.name, func(t *testing.T) {
			var result BaseEvent

			err := json.Unmarshal([]byte(tt.jsonData), &result)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBaseEvent_ImplementsEventInterface(_ *testing.T) {
	var _ Event = (*BaseEvent)(nil)
}

func TestEventBatch(t *testing.T) {
	tests := []struct {
		name  string
		batch EventBatch
		len   int
	}{
		{
			name:  "empty batch",
			batch: EventBatch{},
			len:   0,
		},
		{
			name: "single event batch",
			batch: EventBatch{
				&BaseEvent{EventType: "Event1", Timestmp: 0, EntityKey: ""},
			},
			len: 1,
		},
		{
			name: "multiple events batch",
			batch: EventBatch{
				&BaseEvent{EventType: "Event1", Timestmp: 0, EntityKey: ""},
				&BaseEvent{EventType: "Event2", Timestmp: 0, EntityKey: ""},
				&BaseEvent{EventType: "Event3", Timestmp: 0, EntityKey: ""},
			},
			len: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Len(t, tt.batch, tt.len)
		})
	}
}

func TestBaseEvent_ChainedCalls(t *testing.T) {
	baseEvent := &BaseEvent{EventType: "", Timestmp: 0, EntityKey: ""}
	baseEvent.Type("TestEvent")
	baseEvent.Entity(entity.Key("host:test"))
	baseEvent.Timestamp(1234567890)

	assert.Equal(t, "TestEvent", baseEvent.EventType)
	assert.Equal(t, "host:test", baseEvent.EntityKey)
	assert.Equal(t, int64(1234567890), baseEvent.Timestmp)
}
