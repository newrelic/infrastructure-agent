// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package databind

import (
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/discovery"
)

// cachedEntry allows storing a value for a given Time-To-Leave
type cachedEntry struct {
	ttl    time.Duration
	time   time.Time // time the object has been stored
	stored interface{}
}

//
func (c *cachedEntry) get(now time.Time) (interface{}, bool) {
	if c.stored != nil && c.time.Add(c.ttl).After(now) {
		return c.stored, true
	}
	c.stored = nil
	return nil, false
}

func (c *cachedEntry) set(value interface{}, now time.Time) {
	c.stored = value
	c.time = now
}

// discoverer is any source discovering multiple matches from a source (e.g. containers)
type discoverer struct {
	cache cachedEntry
	// any discovery source must provide a function of this signature
	fetch func() ([]discovery.Discovery, error)
}

func (d *discoverer) do(now time.Time) ([]discovery.Discovery, error) {
	if vals, ok := d.cache.get(now); ok {
		return vals.([]discovery.Discovery), nil
	}
	vals, err := d.fetch()
	if err != nil {
		return nil, err
	}
	d.cache.set(vals, now)
	return vals, nil
}

// gatherer is any source fetching a single match from a variables source (e.g. a vault key)
type gatherer struct {
	cache cachedEntry
	// can return a single string, but also maps or arrays
	fetch func() (interface{}, error)
}

func (d *gatherer) do(now time.Time) (interface{}, error) {
	if vals, ok := d.cache.get(now); ok {
		return vals, nil
	}
	vals, err := d.fetch()
	if err != nil {
		return nil, err
	}
	d.cache.set(vals, now)
	return vals, nil
}
