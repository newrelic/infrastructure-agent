// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package agent

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/newrelic/infrastructure-agent/internal/agent/delta"
	"github.com/newrelic/infrastructure-agent/pkg/backend/inventoryapi"
	"github.com/stretchr/testify/assert"
)

const maxInventoryDataSize = 3 * 1000 * 1000

var plugin = &delta.PluginInfo{
	Source:   "metadata/plugin",
	Plugin:   "metadata",
	FileName: "plugin.json",
}

// createDelta creates and stores a delta JSON for a given entity, with a size approximate to the given size
func createDelta(store *delta.Store, entityKey string, approxKB int) {

	srcFile := store.SourceFilePath(plugin, entityKey)
	err := os.MkdirAll(filepath.Dir(srcFile), 0755)
	if err != nil {
		panic(err)
	}
	bytes := 0

	counter := 0
	diff := map[string]interface{}{}

	for bytes/1024 < approxKB {
		key := fmt.Sprintf("k%049d", counter)
		value := fmt.Sprintf("v%049d", counter)
		if len(key) != 50 || len(value) != 50 {
			panic(fmt.Sprintf("%q and %q should have length 50 and have %d and %d", key, value, len(key), len(value)))
		}
		diff[key] = value
		bytes += 100
		counter++
	}
	diffBytes, err := json.Marshal(diff)
	if err != nil {
		panic(err)
	}
	if len(diffBytes) < approxKB {
		panic(fmt.Sprintf("%d < %d", len(diffBytes), approxKB))
	}

	err = ioutil.WriteFile(srcFile, diffBytes, 0644)
	if err != nil {
		panic(err)
	}

	err = store.UpdatePluginsInventoryCache(entityKey)
	if err != nil {
		panic(err)
	}

	err = store.SaveState()
	if err != nil {
		panic(err)
	}
}

// mockIngestClient mocks the ingest client `sendDelta` method, by accounting the invocations
type mockIngestClient struct {
	invocations [][]inventoryapi.PostDeltaBody
	reset       string
}

func ingestClient() mockIngestClient {
	return mockIngestClient{
		invocations: make([][]inventoryapi.PostDeltaBody, 0),
	}
}

func (i *mockIngestClient) sendDelta(reqs []inventoryapi.PostDeltaBody) ([]inventoryapi.BulkDeltaResponse, error) {
	i.invocations = append(i.invocations, reqs)
	response := make([]inventoryapi.BulkDeltaResponse, 0)
	for _, r := range reqs {
		response = append(response, inventoryapi.BulkDeltaResponse{
			PostDeltaResponse: inventoryapi.PostDeltaResponse{
				Reset: i.reset,
			},
			EntityKeys: r.ExternalKeys,
		})
	}
	return response, nil
}

func TestInventories(t *testing.T) {
	// Given a delta store
	dataDir, err := ioutil.TempDir("", "test_inventories")
	assert.Nil(t, err)
	store := delta.NewStore(dataDir, "default", maxInventoryDataSize)

	// And a set of registered entities
	inv := map[string]*inventory{
		"agent_id":                nil,
		"some_other_cool_entity":  nil,
		"some_other_cool_entity2": nil,
	}

	// And an Inventories processor that submits inventories deltas though an ingest client
	client := ingestClient()
	inventories := NewInventories(store, &context{}, &inventoryapi.IngestClient{}, &inv, "agent_id", false, uint64(100000000), maxInventoryDataSize)
	inventories.send = client.sendDelta

	// And a set of stored delta patches that fit within a single invocation to the ingest service
	for entityKey := range inv {
		createDelta(store, entityKey, 900)
	}

	// When the patches are processed in bulk
	inventories.BulkPatchProcess()

	// Then the ingest client is invoked only once
	assert.Equal(t, 1, len(client.invocations))

	// And all the entities have been sent
	assert.Equal(t, 3, len(client.invocations[0]))

	// Including correct information about entity identifiers
	entities := map[string]inventoryapi.PostDeltaBody{}
	for _, e := range client.invocations[0] {
		entities[e.ExternalKeys[0]] = e
	}
	assert.Contains(t, entities, "agent_id")
	assert.True(t, *entities["agent_id"].IsAgent)
	assert.Contains(t, entities, "some_other_cool_entity")
	assert.False(t, *entities["some_other_cool_entity"].IsAgent)
	assert.Contains(t, entities, "some_other_cool_entity2")
	assert.False(t, *entities["some_other_cool_entity2"].IsAgent)

	// And the deltas buffer has been cleaned up
	assert.Equal(t, 0, inventories.buffer.Entries())

	// But the deltas are still in the store
	_, err = os.Stat(store.DeltaFilePath(plugin, "agent_id"))
	assert.Nil(t, err)
	_, err = os.Stat(store.DeltaFilePath(plugin, "some_other_cool_entity"))
	assert.Nil(t, err)
	_, err = os.Stat(store.DeltaFilePath(plugin, "some_other_cool_entity2"))
	assert.Nil(t, err)
}

func TestInventories_BulkPatchProcess(t *testing.T) {
	// Given a delta store
	dataDir, err := ioutil.TempDir("", "test_inventories")
	assert.Nil(t, err)
	store := delta.NewStore(dataDir, "default", maxInventoryDataSize)

	// And a set of registered entities
	inv := map[string]*inventory{
		"agent_id":                nil,
		"some_other_cool_entity":  nil,
		"some_other_cool_entity2": nil,
		"some_other_cool_entity3": nil,
		"some_other_cool_entity4": nil,
	}

	// And an Inventories processor that submits inventories deltas though an ingest client
	client := ingestClient()
	inventories := NewInventories(store, &context{}, &inventoryapi.IngestClient{}, &inv, "agent_id", false, uint64(100000000), maxInventoryDataSize)
	inventories.send = client.sendDelta

	// And a set of stored delta patches that DON'T fit within a single invocation to the ingest service
	// because capacity is limited by maxInventoryDataSize
	for entityKey := range inv {
		createDelta(store, entityKey, 900)
	}

	// When the patches are processed in bulk
	inventories.BulkPatchProcess()

	// Then the ingest client is invoked twice
	assert.Equal(t, 2, len(client.invocations))

	// And all the entities have been sent
	assert.Equal(t, 3, len(client.invocations[0]))
	assert.Equal(t, 2, len(client.invocations[1]))

	// Including correct information about entity identifiers
	entities := map[string]inventoryapi.PostDeltaBody{}
	for _, e := range client.invocations[0] {
		entities[e.ExternalKeys[0]] = e
	}
	for _, e := range client.invocations[1] {
		entities[e.ExternalKeys[0]] = e
	}

	assert.Contains(t, entities, "agent_id")
	assert.True(t, *entities["agent_id"].IsAgent)
	assert.Contains(t, entities, "some_other_cool_entity")
	assert.False(t, *entities["some_other_cool_entity"].IsAgent)
	assert.Contains(t, entities, "some_other_cool_entity2")
	assert.False(t, *entities["some_other_cool_entity2"].IsAgent)
	assert.Contains(t, entities, "some_other_cool_entity3")
	assert.False(t, *entities["some_other_cool_entity3"].IsAgent)
	assert.Contains(t, entities, "some_other_cool_entity4")
	assert.False(t, *entities["some_other_cool_entity4"].IsAgent)

	// And the deltas buffer has been cleaned up
	assert.Equal(t, 0, inventories.buffer.Entries())
}

func TestInventories_NoDeltas(t *testing.T) {
	// Given a delta store
	dataDir, err := ioutil.TempDir("", "no_deltas")
	assert.Nil(t, err)
	store := delta.NewStore(dataDir, "default", maxInventoryDataSize)

	// And a set of registered entities
	inv := map[string]*inventory{
		"agent_id":                nil,
		"some_other_cool_entity":  nil,
		"some_other_cool_entity2": nil,
	}

	// And an Inventories processor that submits inventories deltas though an ingest client
	client := ingestClient()
	inventories := NewInventories(store, &context{}, &inventoryapi.IngestClient{}, &inv, "agent_id", false, uint64(100000000), maxInventoryDataSize)
	inventories.send = client.sendDelta

	// When the patches are processed, but there are no new deltas to submit
	inventories.BulkPatchProcess()

	// Then the ingest client is not invoked
	assert.Equal(t, 0, len(client.invocations))

	// And the deltas buffer is empty
	assert.Equal(t, 0, inventories.buffer.Entries())

}

func TestInventories_ResetAll(t *testing.T) {
	// Given a delta store
	dataDir, err := ioutil.TempDir("", "reset_all")
	assert.Nil(t, err)
	store := delta.NewStore(dataDir, "default", maxInventoryDataSize)

	// And a set of registered entities
	inv := map[string]*inventory{
		"agent_id":                nil,
		"some_other_cool_entity":  nil,
		"some_other_cool_entity2": nil,
	}

	// And an Inventories processor that submits inventories deltas though an ingest client
	client := ingestClient()
	client.reset = inventoryapi.ResetAll
	inventories := NewInventories(store, &context{reconnecting: new(sync.Map)}, &inventoryapi.IngestClient{}, &inv, "agent_id", false, uint64(100000000), maxInventoryDataSize)
	inventories.send = client.sendDelta

	// And a set of stored delta patches that fit within a single invocation to the ingest service
	for entityKey := range inv {
		createDelta(store, entityKey, 900)
	}
	_, err = os.Stat(store.SourceFilePath(plugin, "agent_id"))
	assert.Nil(t, err)
	_, err = os.Stat(store.SourceFilePath(plugin, "some_other_cool_entity"))
	assert.Nil(t, err)
	_, err = os.Stat(store.SourceFilePath(plugin, "some_other_cool_entity2"))
	assert.Nil(t, err)

	// When the patches are processed in bulk
	inventories.BulkPatchProcess()

	// And the ingest service returns a 'Reset All response", the deltas removed from the store
	_, err = os.Stat(store.DeltaFilePath(plugin, "agent_id"))
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(store.DeltaFilePath(plugin, "some_other_cool_entity"))
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(store.DeltaFilePath(plugin, "some_other_cool_entity2"))
	assert.True(t, os.IsNotExist(err))
}

func TestInventories_Compacting(t *testing.T) {
	// Given a delta store
	dataDir, err := ioutil.TempDir("", "reset_all")
	assert.Nil(t, err)
	store := delta.NewStore(dataDir, "default", maxInventoryDataSize)

	// And a set of registered entities
	inv := map[string]*inventory{
		"agent_id":                nil,
		"some_other_cool_entity":  nil,
		"some_other_cool_entity2": nil,
	}
	for entityKey := range inv {
		createDelta(store, entityKey, 900)
	}

	// And an Inventories processor that submits inventories deltas though an ingest client
	client := ingestClient()
	client.reset = inventoryapi.ResetAll
	inventories := NewInventories(store, &context{reconnecting: new(sync.Map)}, &inventoryapi.IngestClient{}, &inv, "agent_id", true, uint64(50), maxInventoryDataSize)
	inventories.send = client.sendDelta

	beforeCompacting, err := store.StorageSize(store.CacheDir)
	assert.Nil(t, err)
	// When the patches are processed in bulk with the "compact storage" option enabled
	inventories.BulkPatchProcess()

	// Then the storage size of the data directory has been decreased
	afterCompacting, err := store.StorageSize(store.CacheDir)
	assert.Nil(t, err)
	assert.True(t, beforeCompacting > afterCompacting, "%d should be greater than %d", beforeCompacting, afterCompacting)
}
