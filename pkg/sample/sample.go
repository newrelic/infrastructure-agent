// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package sample

import (
	"github.com/newrelic/infrastructure-agent/pkg/entity"
)

// Event is implemented by any object that can represent an event/sample as long as we can marshal it.
type Event interface {
	// Type sets the "eventType" marshallable field
	Type(eventType string)
	// Entity sets the "entityKey" marshallable field
	Entity(key entity.Key)
	// Timestamp sets the "timestamp" marshallable field
	Timestamp(timestamp int64)
}

// EventBatch is a slice of Event
type EventBatch []Event

// BaseEvent type specifying properties for all sample events
// All fields on SampleEvent must be set before it is sent.
type BaseEvent struct {
	EventType string `json:"eventType"`
	Timestmp  int64  `json:"timestamp"`
	EntityKey string `json:"entityKey"`
}

var _ Event = (*BaseEvent)(nil) // BaseEvent implements sample.Event

// Type sets the event type
func (bse *BaseEvent) Type(eventType string) {
	bse.EventType = eventType
}

// Entity sets the event entity
func (bse *BaseEvent) Entity(key entity.Key) {
	bse.EntityKey = string(key)
}

// Timestamp sets the event timestamp
func (bse *BaseEvent) Timestamp(timestamp int64) {
	bse.Timestmp = timestamp
}
