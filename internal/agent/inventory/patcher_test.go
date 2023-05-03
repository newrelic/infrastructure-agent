// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package inventory

import (
	"encoding/json"
	"github.com/newrelic/infrastructure-agent/internal/agent/delta"
	agentTypes "github.com/newrelic/infrastructure-agent/internal/agent/types"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPatcher_NeedsCleanup_NeverCleaned(t *testing.T) {
	b := BasePatcher{}
	assert.False(t, b.needsCleanup())
	assert.False(t, b.needsCleanup())
}

func TestPatcher_NeedsCleanup_DefaultRemoveEntitiesPeriodExceeded(t *testing.T) {
	b := BasePatcher{
		lastClean: time.Now().Add(-(defaultRemoveEntitiesPeriod + 1)),
	}
	assert.True(t, b.needsCleanup())
	assert.False(t, b.needsCleanup())
}

func TestPatcher_NeedsCleanup_ConfigRemoveEntitiesPeriodExceeded(t *testing.T) {
	b := BasePatcher{
		cfg: PatcherConfig{
			RemoveEntitiesPeriod: 1 * time.Hour,
		},
		lastClean: time.Now().Add(-30 * time.Minute),
	}
	assert.False(t, b.needsCleanup())

	b.lastClean = b.lastClean.Add(-(30*time.Minute + 1))
	assert.True(t, b.needsCleanup())
	assert.False(t, b.needsCleanup())
}

type testInventoryData struct {
	Name  string
	Value *string
}

func (t *testInventoryData) SortKey() string {
	return t.Name
}

func TestPatcher_Save(t *testing.T) {
	dataDir, err := ioutil.TempDir("", "prefix")
	require.NoError(t, err)

	deltaStore := delta.NewStore(dataDir, "default", 1024, false)

	b := BasePatcher{
		cfg:        PatcherConfig{},
		deltaStore: deltaStore,
	}
	defer os.RemoveAll(deltaStore.DataDir)

	aV := "aValue"
	bV := "bValue"
	cV := "cValue"

	err = b.save(agentTypes.PluginOutput{
		Id: ids.PluginID{
			Category: "test",
			Term:     "plugin",
		},

		Entity: entity.NewFromNameWithoutID("someEntity"),
		Data: agentTypes.PluginInventoryDataset{
			&testInventoryData{"cMyService", &cV},
			&testInventoryData{"aMyService", &aV},
			&testInventoryData{"NilService", nil},
			&testInventoryData{"bMyService", &bV},
		},
	})

	assert.NoError(t, err)

	sourceFile := filepath.Join(deltaStore.DataDir, "test", "someEntity", "plugin.json")
	sourceB, err := ioutil.ReadFile(sourceFile)
	require.NoError(t, err)

	expected := []byte(`{"NilService":{"Name":"NilService"},"aMyService":{"Name":"aMyService","Value":"aValue"},"bMyService":{"Name":"bMyService","Value":"bValue"},"cMyService":{"Name":"cMyService","Value":"cValue"}}`)

	assert.Equal(t, string(expected), string(sourceB))
}

func TestPatcher_SaveIgnored(t *testing.T) {
	dataDir, err := ioutil.TempDir("", "prefix")
	require.NoError(t, err)

	deltaStore := delta.NewStore(dataDir, "default", 1024, false)

	b := BasePatcher{
		cfg: PatcherConfig{
			IgnoredPaths: map[string]struct{}{"test/plugin/yum": {}},
		},
		deltaStore: deltaStore,
	}
	defer os.RemoveAll(deltaStore.DataDir)

	aV := "aValue"
	bV := "bValue"

	assert.NoError(t, b.save(agentTypes.PluginOutput{
		Id: ids.PluginID{
			Category: "test",
			Term:     "plugin",
		},
		Entity: entity.NewFromNameWithoutID("someEntity"),
		Data: agentTypes.PluginInventoryDataset{
			&testInventoryData{"yum", &aV},
			&testInventoryData{"myService", &bV},
		},
	}))

	restoredDataBytes, err := ioutil.ReadFile(filepath.Join(deltaStore.DataDir, "test", "someEntity", "plugin.json"))
	require.NoError(t, err)

	var restoredData map[string]interface{}
	require.NoError(t, json.Unmarshal(restoredDataBytes, &restoredData))

	assert.Equal(t, restoredData, map[string]interface{}{
		"myService": map[string]interface{}{
			"Name":  "myService",
			"Value": "bValue",
		},
	})
}
