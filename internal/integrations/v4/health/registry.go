// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// Package health implements an in-memory registry holding the latest health
// record for every tracked on-host integration instance.
package health

import "sync"

// HealthEntry is the health state of one integration instance at the last
// time it was evaluated.
type HealthEntry struct {
	Healthy            bool   `json:"healthy"`
	Status             string `json:"status"`
	LastError          string `json:"last_error"`
	StartTimeUnixNano  int64  `json:"start_time_unix_nano"`
	StatusTimeUnixNano int64  `json:"status_time_unix_nano"`
}

// Reader is the read-only half of Registry, so read-only consumers like
// HTTP handlers can't accidentally write.
type Reader interface {
	Get(path, index string) (HealthEntry, bool)
	ListByPath(path string) (map[string]HealthEntry, bool)
	ListAll() map[string]map[string]HealthEntry
}

// Writer is the write-only half of Registry, so write-only consumers like
// the runner goroutines and the config-file watcher can't accidentally
// read-and-branch on unrelated state.
type Writer interface {
	Record(path, index string, entry HealthEntry)
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
	data map[string]map[string]HealthEntry
}

var _ Registry = (*InMemoryRegistry)(nil)

// NewInMemoryRegistry returns an empty InMemoryRegistry.
func NewInMemoryRegistry() *InMemoryRegistry {
	return &InMemoryRegistry{
		data: make(map[string]map[string]HealthEntry),
	}
}

func (r *InMemoryRegistry) Record(path, index string, entry HealthEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.data == nil {
		r.data = make(map[string]map[string]HealthEntry)
	}
	if r.data[path] == nil {
		r.data[path] = make(map[string]HealthEntry)
	}
	r.data[path][index] = entry
}

func (r *InMemoryRegistry) Get(path, index string) (HealthEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	instances, ok := r.data[path]
	if !ok {
		return HealthEntry{}, false
	}
	entry, ok := instances[index]
	return entry, ok
}

// ListByPath returns a copy, safe for the caller to mutate without
// affecting the registry.
func (r *InMemoryRegistry) ListByPath(path string) (map[string]HealthEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	instances, ok := r.data[path]
	if !ok {
		return nil, false
	}

	out := make(map[string]HealthEntry, len(instances))
	for index, entry := range instances {
		out[index] = entry
	}
	return out, true
}

// ListAll returns a copy, safe for the caller to mutate without affecting
// the registry.
func (r *InMemoryRegistry) ListAll() map[string]map[string]HealthEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make(map[string]map[string]HealthEntry, len(r.data))
	for path, instances := range r.data {
		copied := make(map[string]HealthEntry, len(instances))
		for index, entry := range instances {
			copied[index] = entry
		}
		out[path] = copied
	}
	return out
}

// Delete also removes path itself once its last instance is gone, so an
// empty path doesn't linger in ListAll.
func (r *InMemoryRegistry) Delete(path, index string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	instances, ok := r.data[path]
	if !ok {
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
