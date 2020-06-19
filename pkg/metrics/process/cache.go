// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux

package process

import (
	"github.com/newrelic/infrastructure-agent/pkg/helpers/lru"
	"github.com/newrelic/infrastructure-agent/pkg/metrics"
)

// processCache wraps the invocations to lru.Cache, enabling clearer code and type safety
type cache struct {
	items *lru.Cache
}

type cacheEntry struct {
	process    *linuxProcess
	lastSample *metrics.ProcessSample // The last event we generated for this process, so we can re-use metadata which doesn't change
}

func newCache() cache {
	return cache{items: lru.New()}
}

// Add associates a process cache entry to a given PID
func (p *cache) Add(pid int32, process *cacheEntry) {
	p.items.Add(pid, process)
}

// Get returns the process cache entry associated to the given PID
func (p *cache) Get(pid int32) (*cacheEntry, bool) {
	if val, ok := p.items.Get(pid); !ok {
		return nil, false
	} else {
		return val.(*cacheEntry), true
	}
}
