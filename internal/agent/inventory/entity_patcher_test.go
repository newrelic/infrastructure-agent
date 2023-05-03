// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package inventory

import (
	"github.com/newrelic/infrastructure-agent/internal/agent/delta"
	agentTypes "github.com/newrelic/infrastructure-agent/internal/agent/types"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestCleanOutdatedEntities(t *testing.T) {
	dataDir, err := ioutil.TempDir("", "prefix")
	require.NoError(t, err)

	deltaStore := delta.NewStore(dataDir, "default", 1024, false)

	defer os.RemoveAll(deltaStore.DataDir)

	const aPlugin = "aPlugin"
	const anotherPlugin = "anotherPlugin"

	// GIVEN en entity patcher
	entityPatcher := &EntityPatcher{
		BasePatcher: BasePatcher{
			cfg:        PatcherConfig{},
			deltaStore: deltaStore,
		},
		patchSenderProviderFn: func(e entity.Entity) (PatchSender, error) {
			return nil, nil
		},
		entities: map[entity.Key]struct {
			sender       PatchSender
			needsReaping bool
		}{},
		seenEntities: map[entity.Key]struct{}{},
	}

	dataDirPath := entityPatcher.deltaStore.DataDir

	defer os.RemoveAll(dataDirPath)

	// With a set of registered entities
	for _, id := range []string{"entity:1", "entity:2", "entity:3"} {
		entityPatcher.registerEntity(entity.NewFromNameWithoutID(id))

		assert.NoError(t, os.MkdirAll(filepath.Join(dataDir, aPlugin, helpers.SanitizeFileName(id)), 0755))
		assert.NoError(t, os.MkdirAll(filepath.Join(dataDir, anotherPlugin, helpers.SanitizeFileName(id)), 0755))
	}

	// With some entity inventory folders from previous executions
	assert.NoError(t, os.MkdirAll(filepath.Join(dataDir, aPlugin, "entity4"), 0755))
	assert.NoError(t, os.MkdirAll(filepath.Join(dataDir, aPlugin, "entity5"), 0755))
	assert.NoError(t, os.MkdirAll(filepath.Join(dataDir, aPlugin, "entity6"), 0755))
	assert.NoError(t, os.MkdirAll(filepath.Join(dataDir, anotherPlugin, "entity4"), 0755))
	assert.NoError(t, os.MkdirAll(filepath.Join(dataDir, anotherPlugin, "entity5"), 0755))
	assert.NoError(t, os.MkdirAll(filepath.Join(dataDir, anotherPlugin, "entity6"), 0755))

	// WHEN not all the entities reported information during the last period
	entityPatcher.Save(agentTypes.PluginOutput{
		Id: ids.PluginID{
			Category: "test",
			Term:     "plugin",
		},

		Entity: entity.NewFromNameWithoutID("entity:2"),
		Data:   agentTypes.PluginInventoryDataset{},
	})

	// AND the "remove outdated entities" is triggered
	entityPatcher.cleanOutdatedEntities()

	// THEN the entities that didn't reported information have been unregistered
	// and only their folders are kept
	entities := []struct {
		ID             string
		Folder         string
		StillReporting bool
	}{
		{"entity:1", "entity1", false},
		{"entity:2", "entity2", true},
		{"entity:3", "entity3", false},
		{"dontCare", "entity4", false},
		{"doesntMatter", "entity5", false},
	}
	for _, e := range entities {
		_, err1 := os.Stat(filepath.Join(dataDir, aPlugin, e.Folder))
		_, err2 := os.Stat(filepath.Join(dataDir, anotherPlugin, e.Folder))
		if e.StillReporting {
			assert.NoError(t, err1)
			assert.NoError(t, err2)
		} else {
			assert.True(t, os.IsNotExist(err1))
			assert.True(t, os.IsNotExist(err2))
		}
	}
}
