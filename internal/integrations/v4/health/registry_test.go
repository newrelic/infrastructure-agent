// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package health

import (
	"fmt"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testRedisConfigPath = "/etc/redis-config.yaml"

func newTestEntry(healthy bool, status, lastError string) Entry {
	return Entry{
		Healthy:            healthy,
		Status:             status,
		LastError:          lastError,
		StartTimeUnixNano:  0,
		StatusTimeUnixNano: 0,
	}
}

func TestRegistry_RecordAndGet(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		path  string
		index string
		entry Entry
	}{
		{
			name:  "healthy entry",
			path:  "/etc/newrelic-infra/integrations.d/redis-config.yaml",
			index: "1",
			entry: newTestEntry(true, "execution successfully", ""),
		},
		{
			name:  "unhealthy entry",
			path:  "/etc/newrelic-infra/integrations.d/ibmmq-config.yaml",
			index: "2",
			entry: newTestEntry(false, "execution failed", "exit status 1"),
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			registry := NewInMemoryRegistry()
			registry.Record(testCase.path, testCase.index, testCase.entry)

			got, found := registry.Get(testCase.path, testCase.index)
			require.True(t, found)
			assert.Equal(t, testCase.entry, got)
		})
	}
}

func TestRegistry_GetMissing(t *testing.T) {
	t.Parallel()

	registry := NewInMemoryRegistry()

	_, found := registry.Get("/does/not/exist.yaml", "1")
	assert.False(t, found, "unknown path should not be found")

	registry.Record(testRedisConfigPath, "1", newTestEntry(true, "", ""))

	_, found = registry.Get(testRedisConfigPath, "2")
	assert.False(t, found, "unknown index under a known path should not be found")
}

func TestRegistry_ListByPath(t *testing.T) {
	t.Parallel()

	registry := NewInMemoryRegistry()
	path := "/etc/newrelic-infra/integrations.d/redis-config.yaml"
	registry.Record(path, "1", newTestEntry(true, "execution successfully", ""))
	registry.Record(path, "2", newTestEntry(false, "reporting empty data", ""))

	instances, found := registry.ListByPath(path)
	require.True(t, found)
	assert.Len(t, instances, 2)
	assert.True(t, instances["1"].Healthy)
	assert.False(t, instances["2"].Healthy)

	_, found = registry.ListByPath("/unknown.yaml")
	assert.False(t, found)
}

func TestRegistry_ListByPath_ReturnsCopy(t *testing.T) {
	t.Parallel()

	registry := NewInMemoryRegistry()
	registry.Record(testRedisConfigPath, "1", newTestEntry(true, "", ""))

	instances, found := registry.ListByPath(testRedisConfigPath)
	require.True(t, found)

	instances["1"] = newTestEntry(false, "", "")

	got, _ := registry.Get(testRedisConfigPath, "1")
	assert.True(t, got.Healthy, "mutating the returned map must not affect the registry")
}

func TestRegistry_ListAll(t *testing.T) {
	t.Parallel()

	registry := NewInMemoryRegistry()
	registry.Record(testRedisConfigPath, "1", newTestEntry(true, "", ""))
	registry.Record("/etc/ibmmq-config.yaml", "1", newTestEntry(false, "", ""))

	all := registry.ListAll()
	assert.Len(t, all, 2)
	assert.True(t, all[testRedisConfigPath]["1"].Healthy)
	assert.False(t, all["/etc/ibmmq-config.yaml"]["1"].Healthy)
}

func TestRegistry_ListAll_ReturnsCopy(t *testing.T) {
	t.Parallel()

	registry := NewInMemoryRegistry()
	registry.Record(testRedisConfigPath, "1", newTestEntry(true, "", ""))

	all := registry.ListAll()
	all[testRedisConfigPath]["1"] = newTestEntry(false, "", "")
	all["/new-path.yaml"] = map[string]Entry{"1": newTestEntry(false, "", "")}

	got, _ := registry.Get(testRedisConfigPath, "1")
	assert.True(t, got.Healthy, "mutating the returned map must not affect the registry")

	_, found := registry.Get("/new-path.yaml", "1")
	assert.False(t, found, "adding to the returned map must not affect the registry")
}

func TestRegistry_Delete(t *testing.T) {
	t.Parallel()

	registry := NewInMemoryRegistry()
	registry.Record(testRedisConfigPath, "1", newTestEntry(true, "", ""))
	registry.Record(testRedisConfigPath, "2", newTestEntry(false, "", ""))

	registry.Delete(testRedisConfigPath, "1")

	_, found := registry.Get(testRedisConfigPath, "1")
	assert.False(t, found, "deleted index must be unreachable via Get")

	instances, found := registry.ListByPath(testRedisConfigPath)
	require.True(t, found, "sibling index must leave the path tracked")
	assert.Len(t, instances, 1)
	_, hasDeleted := instances["1"]
	assert.False(t, hasDeleted, "deleted index must be unreachable via ListByPath")

	_, hasSibling := instances["2"]
	assert.True(t, hasSibling, "sibling index must be untouched")

	all := registry.ListAll()
	_, hasDeleted = all[testRedisConfigPath]["1"]
	assert.False(t, hasDeleted, "deleted index must be unreachable via ListAll")
}

func TestRegistry_Delete_LastIndexRemovesPath(t *testing.T) {
	t.Parallel()

	registry := NewInMemoryRegistry()
	registry.Record(testRedisConfigPath, "1", newTestEntry(true, "", ""))

	registry.Delete(testRedisConfigPath, "1")

	_, found := registry.ListByPath(testRedisConfigPath)
	assert.False(t, found, "path with no remaining instances must not be tracked")

	all := registry.ListAll()
	_, found = all[testRedisConfigPath]
	assert.False(t, found, "path with no remaining instances must not appear in ListAll")
}

func TestRegistry_Delete_UnknownPathOrIndexIsNoop(t *testing.T) {
	t.Parallel()

	registry := NewInMemoryRegistry()
	registry.Record(testRedisConfigPath, "1", newTestEntry(true, "", ""))

	assert.NotPanics(t, func() {
		registry.Delete("/unknown.yaml", "1")
		registry.Delete(testRedisConfigPath, "99")
	})

	_, found := registry.Get(testRedisConfigPath, "1")
	assert.True(t, found, "unrelated deletes must not affect existing entries")
}

func TestRegistry_DeletePath(t *testing.T) {
	t.Parallel()

	registry := NewInMemoryRegistry()
	registry.Record(testRedisConfigPath, "1", newTestEntry(true, "", ""))
	registry.Record(testRedisConfigPath, "2", newTestEntry(false, "", ""))
	registry.Record("/etc/ibmmq-config.yaml", "1", newTestEntry(true, "", ""))

	registry.DeletePath(testRedisConfigPath)

	_, found := registry.Get(testRedisConfigPath, "1")
	assert.False(t, found)

	_, found = registry.Get(testRedisConfigPath, "2")
	assert.False(t, found)

	_, found = registry.ListByPath(testRedisConfigPath)
	assert.False(t, found, "every index under a deleted path must be unreachable")

	all := registry.ListAll()
	assert.Len(t, all, 1, "unrelated paths must be unaffected")
	_, found = all["/etc/ibmmq-config.yaml"]
	assert.True(t, found)
}

func TestRegistry_DeletePath_Unknown(t *testing.T) {
	t.Parallel()

	registry := NewInMemoryRegistry()

	assert.NotPanics(t, func() {
		registry.DeletePath("/unknown.yaml")
	})
}

func TestRegistry_ZeroValueIsUsable(t *testing.T) {
	t.Parallel()

	var registry InMemoryRegistry

	assert.NotPanics(t, func() {
		registry.Record(testRedisConfigPath, "1", newTestEntry(true, "", ""))
	}, "a zero-value InMemoryRegistry must self-initialize on first write")

	got, found := registry.Get(testRedisConfigPath, "1")
	require.True(t, found)
	assert.True(t, got.Healthy)
}

func TestRegistry_SatisfiesNarrowInterfaces(t *testing.T) {
	t.Parallel()

	inMemory := NewInMemoryRegistry()

	var (
		asReader   Reader   = inMemory
		asWriter   Writer   = inMemory
		asRegistry Registry = inMemory
	)

	asWriter.Record(testRedisConfigPath, "1", newTestEntry(true, "", ""))

	got, found := asReader.Get(testRedisConfigPath, "1")
	require.True(t, found)
	assert.True(t, got.Healthy)
	assert.NotNil(t, asRegistry)
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	registry := NewInMemoryRegistry()

	const (
		workerCount  = 50
		opsPerWorker = 100
	)

	var waitGroup sync.WaitGroup

	for worker := range workerCount {
		waitGroup.Add(1)

		go func(worker int) {
			defer waitGroup.Done()

			path := fmt.Sprintf("/etc/integration-%d.yaml", worker%5)

			for iteration := range opsPerWorker {
				index := strconv.Itoa(iteration%3 + 1)
				registry.Record(path, index, Entry{
					Healthy:            iteration%2 == 0,
					Status:             "",
					LastError:          "",
					StartTimeUnixNano:  0,
					StatusTimeUnixNano: int64(iteration),
				})
				registry.Get(path, index)
				registry.ListByPath(path)
				registry.ListAll()

				if iteration%10 == 0 {
					registry.Delete(path, index)
				}
			}
		}(worker)
	}

	waitGroup.Wait()
}
