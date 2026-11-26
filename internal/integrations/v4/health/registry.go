// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package health implements an in-memory registry holding the latest health
// record for every tracked on-host integration instance.
package health

import (
	"maps"
	"sync"
)

// Entry is the health state of one integration instance at the last time
// it was evaluated.
type Entry struct {
	Healthy            bool   `json:"healthy"`
	Status             string `json:"status"`
	LastError          string `json:"last_error"`
	StartTimeUnixNano  int64  `json:"start_time_unix_nano"`
	StatusTimeUnixNano int64  `json:"status_time_unix_nano"`
}

// Reader is the read-only half of Registry, so read-only consumers like
// HTTP handlers can't accidentally write.
type Reader interface {
	Get(path, index string) (Entry, bool)
	ListByPath(path string) (map[string]Entry, bool)
	ListAll() map[string]map[string]Entry
}

// Writer is the write-only half of Registry, so write-only consumers like
// the runner goroutines and the config-file watcher can't accidentally
// read-and-branch on unrelated state.
type Writer interface {
	Record(path, index string, entry Entry)
	Delete(path, index string)
	DeletePath(path string)
}

// Registry is the full read-write contract a health store satisfies.
// Wiring code that constructs the store should depend on this; most other
// consumers should depend on the narrower Reader or Writer instead.
type Registry interface {
	Reader
	Writer
}

// InMemoryRegistry is a Registry backed by a process-local map. It holds no
// state beyond the current process, and is cleared when the agent
// restarts. The zero value is safe to use.
type InMemoryRegistry struct {
	mu   sync.RWMutex
	data map[string]map[string]Entry
}

var _ Registry = (*InMemoryRegistry)(nil)

// NewInMemoryRegistry returns an empty InMemoryRegistry.
func NewInMemoryRegistry() *InMemoryRegistry {
	return &InMemoryRegistry{
		mu:   sync.RWMutex{},
		data: make(map[string]map[string]Entry),
	}
}

func (r *InMemoryRegistry) Record(path, index string, entry Entry) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.data == nil {
		r.data = make(map[string]map[string]Entry)
	}

	if r.data[path] == nil {
		r.data[path] = make(map[string]Entry)
	}

	r.data[path][index] = entry
}

func (r *InMemoryRegistry) Get(path, index string) (Entry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	instances, found := r.data[path]
	if !found {
		var empty Entry

		return empty, false
	}

	entry, found := instances[index]

	return entry, found
}

// ListByPath returns a copy, safe for the caller to mutate without
// affecting the registry.
func (r *InMemoryRegistry) ListByPath(path string) (map[string]Entry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	instances, found := r.data[path]
	if !found {
		return nil, false
	}

	out := make(map[string]Entry, len(instances))
	maps.Copy(out, instances)

	return out, true
}

// ListAll returns a copy, safe for the caller to mutate without affecting
// the registry.
func (r *InMemoryRegistry) ListAll() map[string]map[string]Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make(map[string]map[string]Entry, len(r.data))

	for path, instances := range r.data {
		copied := make(map[string]Entry, len(instances))
		maps.Copy(copied, instances)

		out[path] = copied
	}

	return out
}

// Delete also removes path itself once its last instance is gone, so an
// empty path doesn't linger in ListAll.
func (r *InMemoryRegistry) Delete(path, index string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	instances, found := r.data[path]
	if !found {
		return
	}

	delete(instances, index)

	if len(instances) == 0 {
		delete(r.data, path)
	}
}

// DeletePath is used both when path's config file is deleted outright, and
// to clear path's instances ahead of re-indexing on a config reload.
func (r *InMemoryRegistry) DeletePath(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.data, path)
}
