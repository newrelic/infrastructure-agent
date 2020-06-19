// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package entity

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func setNow(day, hour, minute int) {
	now = func() time.Time {
		return time.Date(2019, 1, day, hour, minute, 0, 0, time.Local)
	}
}

func TestKnownIDs_Put(t *testing.T) {
	// Given a Key to IDs map
	kn := NewKnownIDs()

	// When adding entity key <-> ID entries
	kn.Put("entity-1", 12345)
	kn.Put("entity-2", 54321)

	// The IDs can be retrieved from the keys
	id, ok := kn.Get("entity-1")
	assert.True(t, ok)
	assert.EqualValues(t, id, 12345)

	id, ok = kn.Get("entity-2")
	assert.True(t, ok)
	assert.EqualValues(t, id, 54321)
}

func TestKnownIDs_Put_ExpiredEntry(t *testing.T) {
	// Given a Key to IDs map
	kn := NewKnownIDs()

	// that has entries added at different times
	setNow(0, 00, 00)
	kn.Put("entity-1", 12345)

	setNow(0, 10, 00)
	kn.Put("entity-2", 54321)

	// When retrieving ids in the future
	setNow(1, 05, 00)

	// The expired entries are not returned
	_, ok := kn.Get("entity-1")
	assert.False(t, ok)

	// And the alive entries are returned
	id, ok := kn.Get("entity-2")
	assert.True(t, ok)
	assert.EqualValues(t, id, 54321)
}

func TestKnownIDs_Get_UpdatesExpiration(t *testing.T) {
	// Given a Key to IDs map
	kn := NewKnownIDs()

	// With an entry that should expire 24h after being inserted (default TTL)
	setNow(0, 00, 00)
	kn.Put("entity-1", 12345)

	// When the entity is accessed before this 24h
	setNow(0, 12, 00)
	id, ok := kn.Get("entity-1")
	assert.True(t, ok)
	assert.EqualValues(t, id, 12345)

	// The expiration time is upgraded to the last access time + ttl
	setNow(1, 05, 00)
	id, ok = kn.Get("entity-1")
	assert.True(t, ok)
	assert.EqualValues(t, id, 12345)

	// And does not expire until it passes 24 hours since the last access
	setNow(2, 05, 01)
	_, ok = kn.Get("entity-1")
	assert.False(t, ok)
}

func TestKnownIDs_PutType(t *testing.T) {
	// Given a Key to IDs map
	kn := NewKnownIDs()

	// With different TTLs for the registered entity types
	setNow(0, 00, 00)
	kn.SetTTL("container", 1*time.Hour)
	kn.SetTTL("host", 12*time.Hour)

	// And registered entities of different types
	kn.PutType("container", "cnt-1", 1)
	kn.PutType("host", "hst-1", 2)
	kn.PutType("datacenter", "dc-1", 3)

	// The entities are accessible before they expire
	setNow(0, 00, 59)
	id, ok := kn.Get("cnt-1")
	assert.True(t, ok)
	assert.EqualValues(t, id, 1)

	id, ok = kn.Get("hst-1")
	assert.True(t, ok)
	assert.EqualValues(t, id, 2)

	id, ok = kn.Get("dc-1")
	assert.True(t, ok)
	assert.EqualValues(t, id, 3)

	// And they expire at different times, according to their specific TTLs
	setNow(0, 02, 01)
	_, ok = kn.Get("cnt-1") // Container expires after 1 hour since the last access
	assert.False(t, ok)

	id, ok = kn.Get("hst-1")
	assert.True(t, ok)
	assert.EqualValues(t, id, 2)

	id, ok = kn.Get("dc-1")
	assert.True(t, ok)
	assert.EqualValues(t, id, 3)

	setNow(0, 15, 00)
	_, ok = kn.Get("hst-1") // Host expires after 12 hours since the last access
	assert.False(t, ok)

	id, ok = kn.Get("dc-1")
	assert.True(t, ok)
	assert.EqualValues(t, id, 3)

	setNow(1, 15, 01)
	_, ok = kn.Get("dc-1") // Host expires after 24 hours since the last access (default TTL)
	assert.False(t, ok)
}

func TestKnownIDs_CleanOld(t *testing.T) {
	// Given a Key to IDs map
	kn := NewKnownIDs()

	// With entities registered at different times
	setNow(0, 00, 00)
	kn.Put("entity-1", 12345)
	setNow(0, 10, 00)
	kn.Put("entity-2", 54321)

	assert.Len(t, kn.ids, 2)

	// When the CleanOld method is invoked
	setNow(1, 05, 00)
	kn.CleanOld()

	// The expired entries have been removed (to save space)
	assert.Len(t, kn.ids, 1)

	// And the non-expired entries are accessible
	id, ok := kn.Get("entity-2")
	assert.True(t, ok)
	assert.EqualValues(t, id, 54321)
}

func TestKnownIDs_CleanOld_DontChangeExpirations(t *testing.T) {
	// Given a Key to IDs map
	kn := NewKnownIDs()

	// And an entry
	setNow(0, 00, 00)
	kn.Put("entity-1", 12345)

	// When the CleanOld method is invoked at a given time
	setNow(0, 05, 00)
	kn.CleanOld()

	// The expiration time of the surviving entity has not changed
	setNow(1, 00, 01)
	_, ok := kn.Get("entity-1")
	assert.False(t, ok)
}
