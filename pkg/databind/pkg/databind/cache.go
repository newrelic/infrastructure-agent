// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package databind

import (
	"errors"
	"fmt"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/discovery"
	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/secrets"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

// cachedEntry allows storing a value for a given Time-To-Live.
type cachedEntry struct {
	ttl    time.Duration
	time   time.Time // time the object has been stored
	stored interface{}
}

func (c *cachedEntry) getExpirationTime() time.Time {
	return c.time.Add(c.ttl)
}

func (c *cachedEntry) get(now time.Time) (interface{}, bool) {
	if c.stored != nil && c.getExpirationTime().After(now) {
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

type DiscovererType string

const (
	typeDocker  DiscovererType = "docker"
	typeFargate DiscovererType = "fargate"
	typeCmd     DiscovererType = "command"
)

// DiscovererInfo keeps util info about the discoverer.
type DiscovererInfo struct {
	Type     DiscovererType
	Name     string
	Matchers map[string]string
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

	if dataWithTTL, ok := vals.(ValuesWithTTL); ok {
		ttl, err := dataWithTTL.TTL()
		if err != nil {
			if errors.Is(err, secrets.ErrTTLNotFound) {
				// infra-agent will start even when TTL is not provided
				log.Warn(fmt.Printf(
					"%s. Using Default TTL (%s)",
					secrets.ErrTTLNotFound,
					time.Duration(defaultVariablesTTL)*time.Second
				))
				d.cache.ttl = defaultVariablesTTL
			} else {
				return nil, fmt.Errorf("invalid gathered TTL: %w", err) //nolint:wrapcheck
			}
		} else {
			d.cache.ttl = ttl
		}

		valuesWithTTL, err := dataWithTTL.Data()
		if err != nil {
			return nil, fmt.Errorf("invalid gathered Data: %w", err) //nolint:wrapcheck
		}
		vals = valuesWithTTL
	}

	d.cache.set(vals, now)
	return vals, nil
}
