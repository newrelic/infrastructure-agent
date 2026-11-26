// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package health

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_RecordAndGet(t *testing.T) {
	cases := []struct {
		name  string
		path  string
		index string
		entry HealthEntry
	}{
		{
			name:  "healthy entry",
			path:  "/etc/newrelic-infra/integrations.d/redis-config.yaml",
			index: "1",
			entry: HealthEntry{Healthy: true, Status: "execution successfully"},
		},
		{
			name:  "unhealthy entry",
			path:  "/etc/newrelic-infra/integrations.d/ibmmq-config.yaml",
			index: "2",
			entry: HealthEntry{Healthy: false, Status: "execution failed", LastError: "exit status 1"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewInMemoryRegistry()
			r.Record(tc.path, tc.index, tc.entry)

			got, ok := r.Get(tc.path, tc.index)
			require.True(t, ok)
			assert.Equal(t, tc.entry, got)
		})
	}
}

func TestRegistry_GetMissing(t *testing.T) {
	r := NewInMemoryRegistry()

	_, ok := r.Get("/does/not/exist.yaml", "1")
	assert.False(t, ok, "unknown path should not be found")

	r.Record("/etc/redis-config.yaml", "1", HealthEntry{Healthy: true})
	_, ok = r.Get("/etc/redis-config.yaml", "2")
	assert.False(t, ok, "unknown index under a known path should not be found")
}

func TestRegistry_ListByPath(t *testing.T) {
	r := NewInMemoryRegistry()
	path := "/etc/newrelic-infra/integrations.d/redis-config.yaml"
	r.Record(path, "1", HealthEntry{Healthy: true, Status: "execution successfully"})
	r.Record(path, "2", HealthEntry{Healthy: false, Status: "reporting empty data"})

	instances, ok := r.ListByPath(path)
	require.True(t, ok)
	assert.Len(t, instances, 2)
	assert.True(t, instances["1"].Healthy)
	assert.False(t, instances["2"].Healthy)

	_, ok = r.ListByPath("/unknown.yaml")
	assert.False(t, ok)
}

func TestRegistry_ListByPath_ReturnsCopy(t *testing.T) {
	r := NewInMemoryRegistry()
	path := "/etc/redis-config.yaml"
	r.Record(path, "1", HealthEntry{Healthy: true})

	instances, ok := r.ListByPath(path)
	require.True(t, ok)
	instances["1"] = HealthEntry{Healthy: false}

	got, _ := r.Get(path, "1")
	assert.True(t, got.Healthy, "mutating the returned map must not affect the registry")
}

func TestRegistry_ListAll(t *testing.T) {
	r := NewInMemoryRegistry()
	r.Record("/etc/redis-config.yaml", "1", HealthEntry{Healthy: true})
	r.Record("/etc/ibmmq-config.yaml", "1", HealthEntry{Healthy: false})

	all := r.ListAll()
	assert.Len(t, all, 2)
	assert.True(t, all["/etc/redis-config.yaml"]["1"].Healthy)
	assert.False(t, all["/etc/ibmmq-config.yaml"]["1"].Healthy)
}

func TestRegistry_ListAll_ReturnsCopy(t *testing.T) {
	r := NewInMemoryRegistry()
	path := "/etc/redis-config.yaml"
	r.Record(path, "1", HealthEntry{Healthy: true})

	all := r.ListAll()
	all[path]["1"] = HealthEntry{Healthy: false}
	all["/new-path.yaml"] = map[string]HealthEntry{"1": {Healthy: false}}

	got, _ := r.Get(path, "1")
	assert.True(t, got.Healthy, "mutating the returned map must not affect the registry")
	_, ok := r.Get("/new-path.yaml", "1")
	assert.False(t, ok, "adding to the returned map must not affect the registry")
}

func TestRegistry_Delete(t *testing.T) {
	r := NewInMemoryRegistry()
	path := "/etc/redis-config.yaml"
	r.Record(path, "1", HealthEntry{Healthy: true})
	r.Record(path, "2", HealthEntry{Healthy: false})

	r.Delete(path, "1")

	_, ok := r.Get(path, "1")
	assert.False(t, ok, "deleted index must be unreachable via Get")

	instances, ok := r.ListByPath(path)
	require.True(t, ok, "sibling index must leave the path tracked")
	assert.Len(t, instances, 1)
	_, hasDeleted := instances["1"]
	assert.False(t, hasDeleted, "deleted index must be unreachable via ListByPath")
	_, hasSibling := instances["2"]
	assert.True(t, hasSibling, "sibling index must be untouched")

	all := r.ListAll()
	_, hasDeleted = all[path]["1"]
	assert.False(t, hasDeleted, "deleted index must be unreachable via ListAll")
}

func TestRegistry_Delete_LastIndexRemovesPath(t *testing.T) {
	r := NewInMemoryRegistry()
	path := "/etc/redis-config.yaml"
	r.Record(path, "1", HealthEntry{Healthy: true})

	r.Delete(path, "1")

	_, ok := r.ListByPath(path)
	assert.False(t, ok, "path with no remaining instances must not be tracked")

	all := r.ListAll()
	_, ok = all[path]
	assert.False(t, ok, "path with no remaining instances must not appear in ListAll")
}

func TestRegistry_Delete_UnknownPathOrIndexIsNoop(t *testing.T) {
	r := NewInMemoryRegistry()
	r.Record("/etc/redis-config.yaml", "1", HealthEntry{Healthy: true})

	assert.NotPanics(t, func() {
		r.Delete("/unknown.yaml", "1")
		r.Delete("/etc/redis-config.yaml", "99")
	})

	_, ok := r.Get("/etc/redis-config.yaml", "1")
	assert.True(t, ok, "unrelated deletes must not affect existing entries")
}

func TestRegistry_DeletePath(t *testing.T) {
	r := NewInMemoryRegistry()
	r.Record("/etc/redis-config.yaml", "1", HealthEntry{Healthy: true})
	r.Record("/etc/redis-config.yaml", "2", HealthEntry{Healthy: false})
	r.Record("/etc/ibmmq-config.yaml", "1", HealthEntry{Healthy: true})

	r.DeletePath("/etc/redis-config.yaml")

	_, ok := r.Get("/etc/redis-config.yaml", "1")
	assert.False(t, ok)
	_, ok = r.Get("/etc/redis-config.yaml", "2")
	assert.False(t, ok)
	_, ok = r.ListByPath("/etc/redis-config.yaml")
	assert.False(t, ok, "every index under a deleted path must be unreachable")

	all := r.ListAll()
	assert.Len(t, all, 1, "unrelated paths must be unaffected")
	_, ok = all["/etc/ibmmq-config.yaml"]
	assert.True(t, ok)
}

func TestRegistry_DeletePath_Unknown(t *testing.T) {
	r := NewInMemoryRegistry()
	assert.NotPanics(t, func() {
		r.DeletePath("/unknown.yaml")
	})
}

func TestRegistry_ZeroValueIsUsable(t *testing.T) {
	var r InMemoryRegistry

	assert.NotPanics(t, func() {
		r.Record("/etc/redis-config.yaml", "1", HealthEntry{Healthy: true})
	}, "a zero-value InMemoryRegistry must self-initialize on first write")

	got, ok := r.Get("/etc/redis-config.yaml", "1")
	require.True(t, ok)
	assert.True(t, got.Healthy)
}

func TestRegistry_SatisfiesNarrowInterfaces(t *testing.T) {
	r := NewInMemoryRegistry()

	var reader Reader = r
	var writer Writer = r
	var registry Registry = r

	writer.Record("/etc/redis-config.yaml", "1", HealthEntry{Healthy: true})
	got, ok := reader.Get("/etc/redis-config.yaml", "1")
	require.True(t, ok)
	assert.True(t, got.Healthy)
	assert.NotNil(t, registry)
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	r := NewInMemoryRegistry()
	const goroutines = 50
	const opsPerGoroutine = 100

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			path := fmt.Sprintf("/etc/integration-%d.yaml", g%5)
			for i := 0; i < opsPerGoroutine; i++ {
				index := fmt.Sprintf("%d", i%3+1)
				r.Record(path, index, HealthEntry{Healthy: i%2 == 0, StatusTimeUnixNano: int64(i)})
				r.Get(path, index)
				r.ListByPath(path)
				r.ListAll()
				if i%10 == 0 {
					r.Delete(path, index)
				}
			}
		}(g)
	}
	wg.Wait()
}
