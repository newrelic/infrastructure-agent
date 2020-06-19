// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package entity

import "time"

var now = time.Now

const defaultTTL = 24 * time.Hour

// KnownIDs maps the entity IDs given their respective entity Keys.
//
// It associates a TTL to each entry, which expires after a given time without being accesses (both for setting and for
// reading values).
//
// The default TTL is 24h, but it is possible to register custom TTLs per entity type.
//
// This component is not thread-safe.
type KnownIDs struct {
	ids  map[Key]*idEntry
	ttls map[Type]time.Duration // per-entity ttl
}

type idEntry struct {
	id         ID
	lastAccess time.Time
	ttl        time.Duration
}

func (e *idEntry) isOutdated() bool {
	return e.lastAccess.Add(e.ttl).Before(now())
}

// NewKnownIDs creates and returns an empty KnownIDs map
func NewKnownIDs() KnownIDs {
	return KnownIDs{
		ids:  map[Key]*idEntry{},
		ttls: map[Type]time.Duration{},
	}
}

// Put registers an entity ID for a given entity Key. The entry has a default TTL of 24 hours.
func (k *KnownIDs) Put(key Key, id ID) {
	k.putTTL(key, id, defaultTTL)
}

// PutType registers an entity ID for a given entity Key of the given Type. The new entry has a TTL according to the
// type registered with the SetTTL function. If the entity type has not been registered, it assumes the default TTL of
// 24 hours.
func (k *KnownIDs) PutType(entityType Type, key Key, id ID) {
	if ttl, ok := k.ttls[entityType]; ok {
		k.putTTL(key, id, ttl)
	} else {
		k.putTTL(key, id, defaultTTL)
	}
}

func (k *KnownIDs) putTTL(key Key, id ID, ttl time.Duration) {
	k.ids[key] = &idEntry{
		id:         id,
		lastAccess: now(),
		ttl:        ttl,
	}
}

// Get returns the entity ID for the given entity Key, if exists. If the entry is found, its expiration time is updated
// to the current time + TTL.
func (k *KnownIDs) Get(key Key) (ID, bool) {
	entry, ok := k.ids[key]
	if !ok {
		return 0, false
	}
	// If the TTL expired, we remove the entry and return as not found
	if entry.isOutdated() {
		delete(k.ids, key)
		return 0, false
	}
	entry.lastAccess = now()
	return entry.id, true
}

// SetTTL registers a custom TTL for the given entity Type
func (k *KnownIDs) SetTTL(entityType Type, ttl time.Duration) {
	k.ttls[entityType] = ttl
}

// Clean removes the expired Key <-> ID entries
func (kn *KnownIDs) CleanOld() {
	for k, e := range kn.ids {
		if e.isOutdated() {
			delete(kn.ids, k)
		}
	}
}
