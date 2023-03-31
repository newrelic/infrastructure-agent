// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package delta

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/infrastructure-agent/pkg/backend/inventoryapi"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
)

const maxInventorySize = 3 * 1000 * 1000

func TempDeltaStoreDir() (string, error) {
	return ioutil.TempDir("", "deltastore")
}

func TestRemoveNulls(t *testing.T) {
	obj := map[string]interface{}{
		"child1": map[string]interface{}{
			"attr1": "foo",
			"attr2": 1,
			"attr3": nil,
		},
		"child2": []interface{}{
			"foo",
			1234,
			map[string]interface{}{
				"id":   1,
				"name": nil,
			},
		},
		"child3": nil,
	}

	removeNils(obj)

	child1 := obj["child1"].(map[string]interface{})
	_, hasChild1Attr3 := child1["attr3"]
	assert.False(t, hasChild1Attr3)

	child2 := obj["child2"].([]interface{})
	child2Map := child2[2].(map[string]interface{})
	_, hasChild2MapName := child2Map["name"]
	assert.False(t, hasChild2MapName)

	_, hasChild3 := obj["child3"]
	assert.False(t, hasChild3)
}

func TestPluginInfoNextDelta(t *testing.T) {
	pi := &PluginInfo{}

	const eKey = "entity_key"
	pi.increaseDeltaID(eKey)
	assert.Equal(t, int64(1), pi.deltaID(eKey))
	pi.increaseDeltaID(eKey)
	assert.Equal(t, int64(2), pi.deltaID(eKey))
}

func TestNewDeltaStoreGolden(t *testing.T) {
	dataDir, err := TempDeltaStoreDir()
	require.NoError(t, err)
	defer os.RemoveAll(dataDir)

	repoDir := filepath.Join(dataDir, "delta")
	ds := NewStore(repoDir, "default", maxInventorySize, true)
	assert.NotNil(t, ds)
}

func TestStorageSize(t *testing.T) {
	dataDir, err := TempDeltaStoreDir()
	require.NoError(t, err)
	defer os.RemoveAll(dataDir)

	repoDir := filepath.Join(dataDir, "delta")
	ds := NewStore(repoDir, "default", maxInventorySize, true)

	size, _ := ds.StorageSize(ds.CacheDir)
	assert.Equal(t, uint64(0), size)

	deltaFilePath := filepath.Join(ds.CacheDir, "test")
	f, err := os.OpenFile(deltaFilePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	require.NoError(t, err)
	defer f.Close()

	buf := []byte{'1', 'A', '#'}
	_, err = f.Write(buf)
	assert.NoError(t, err)

	size, _ = ds.StorageSize(ds.CacheDir)
	assert.Equal(t, uint64(len(buf)), size)
}

type DeltaUtilsCoreSuite struct {
	dataDir  string
	repoDir  string
	plugin   *PluginInfo
	pluginID ids.PluginID
}

func (d DeltaUtilsCoreSuite) TearDownTest() {
	_ = os.RemoveAll(d.dataDir)
}

func SetUpTest(t *testing.T) (d DeltaUtilsCoreSuite) {
	var err error
	d.dataDir, err = TempDeltaStoreDir()
	require.NoError(t, err)
	d.repoDir = filepath.Join(d.dataDir, "delta")
	d.plugin = newPluginInfo("metadata", "plugin.json")
	d.pluginID = ids.PluginID{
		Category: "metadata",
		Term:     "plugin",
	}
	return d
}

func assertSuffix(t *testing.T, expected string, actual string) bool {
	return assert.True(t,
		strings.HasSuffix(actual, expected),
		fmt.Sprintf("%s should have sufix %s", actual, expected))
}

func TestArchiveFilePath(t *testing.T) {
	s := SetUpTest(t)
	defer s.TearDownTest()
	ds := NewStore(s.repoDir, "default", maxInventorySize, true)

	actual := ds.archiveFilePath(s.plugin, "entity:id")
	expected := filepath.Join("delta", ".delta_repo", "metadata", "entityid", "plugin.sent")
	assertSuffix(t, expected, actual)
}

func TestDeltaFilePath(t *testing.T) {
	s := SetUpTest(t)
	defer s.TearDownTest()
	ds := NewStore(s.repoDir, "default", maxInventorySize, true)

	actual := ds.DeltaFilePath(s.plugin, "entity:id:2")
	expected := filepath.Join("delta", ".delta_repo", "metadata", "entityid2", "plugin.pending")
	assertSuffix(t, expected, actual)
}

func TestCachedFilePath(t *testing.T) {
	s := SetUpTest(t)
	defer s.TearDownTest()
	ds := NewStore(s.repoDir, "default", maxInventorySize, true)

	actual := ds.cachedFilePath(s.plugin, "hello!!everybody")
	expected := filepath.Join("delta", ".delta_repo", "metadata", "hello!!everybody", "plugin.json")
	assertSuffix(t, expected, actual)
}

func TestSourceFilePath(t *testing.T) {
	s := SetUpTest(t)
	defer s.TearDownTest()
	ds := NewStore(s.repoDir, "default", maxInventorySize, true)

	actual := ds.SourceFilePath(s.plugin, "xxxx")
	expected := filepath.Join("delta", "metadata", "xxxx", "plugin.json")
	assertSuffix(t, expected, actual)
}

func TestArchiveFilePath_localEntity(t *testing.T) {
	s := SetUpTest(t)
	defer s.TearDownTest()
	ds := NewStore(s.repoDir, "my-hostname", maxInventorySize, true)

	actual := ds.archiveFilePath(s.plugin, "")
	expected := filepath.Join("delta", ".delta_repo", "metadata", localEntityFolder, "plugin.sent")
	assertSuffix(t, expected, actual)
}

func TestDeltaFilePath_localEntity(t *testing.T) {
	s := SetUpTest(t)
	defer s.TearDownTest()
	ds := NewStore(s.repoDir, "my-hostname", maxInventorySize, true)

	actual := ds.DeltaFilePath(s.plugin, "")
	expected := filepath.Join("delta", ".delta_repo", "metadata", localEntityFolder, "plugin.pending")
	assertSuffix(t, expected, actual)
}

func TestCachedFilePath_localEntity(t *testing.T) {
	s := SetUpTest(t)
	defer s.TearDownTest()
	ds := NewStore(s.repoDir, "my-hostname", maxInventorySize, true)

	actual := ds.cachedFilePath(s.plugin, "")
	expected := filepath.Join("delta", ".delta_repo", "metadata", localEntityFolder, "plugin.json")
	assertSuffix(t, expected, actual)
}

func TestSourceFilePath_localEntity(t *testing.T) {
	s := SetUpTest(t)
	defer s.TearDownTest()
	ds := NewStore(s.repoDir, "my-hostname", maxInventorySize, true)

	actual := ds.SourceFilePath(s.plugin, "")
	expected := filepath.Join("delta", "metadata", localEntityFolder, "plugin.json")
	assertSuffix(t, expected, actual)
}

func TestBaseDirectories(t *testing.T) {
	s := SetUpTest(t)
	defer s.TearDownTest()
	ds := NewStore(s.repoDir, "default", maxInventorySize, true)

	assert.Equal(t, s.repoDir, ds.DataDir)
	assert.Equal(t, filepath.Join(s.repoDir, CACHE_DIR), ds.CacheDir)
}

func TestResetAllSentDeltas(t *testing.T) {
	s := SetUpTest(t)
	defer s.TearDownTest()
	const eKey = "entityKey"
	ds := NewStore(s.repoDir, "default", maxInventorySize, true)
	srcFile := ds.SourceFilePath(s.plugin, eKey)
	err := os.MkdirAll(filepath.Dir(srcFile), 0755)
	require.NoError(t, err)
	diff := []byte(`{"hostname":{"alias":"eee-opsmatic","id":"hostname"}}`)
	err = ioutil.WriteFile(srcFile, diff, 0644)
	require.NoError(t, err)
	_, err = os.Stat(ds.cachedFilePath(s.plugin, eKey))
	require.Error(t, err)
	updated, err := ds.updatePluginInventoryCache(s.plugin, eKey)
	require.NoError(t, err)
	assert.True(t, updated)
	assert.Equal(t, int64(0), ds.plugins["metadata/plugin"].lastSentID(eKey))
	assert.Equal(t, int64(1), ds.plugins["metadata/plugin"].deltaID(eKey))
	_, err = os.Stat(ds.cachedFilePath(s.plugin, eKey))
	require.NoError(t, err)

	ds.ResetAllDeltas(eKey)

	_, err = os.Stat(ds.cachedFilePath(s.plugin, eKey))
	require.Error(t, err)
}

func TestUpdateLastDeltaSentNoHint(t *testing.T) {
	s := SetUpTest(t)
	defer s.TearDownTest()
	ds := NewStore(s.repoDir, "default", maxInventorySize, true)
	s.plugin.setDeltaID("entityKey", 1)
	ds.plugins["metadata/plugin"] = s.plugin
	diff := make(map[string]interface{})
	diff["test"] = "value"
	delta := &inventoryapi.RawDelta{
		Source:    "metadata/plugin",
		ID:        int64(2),
		Timestamp: int64(1),
		Diff:      diff,
	}

	dsm := make(inventoryapi.DeltaStateMap)
	deltaArr := []*inventoryapi.RawDelta{delta}
	ds.UpdateState("entityKey", deltaArr, &dsm)

	assert.Equal(t, int64(2), ds.plugins["metadata/plugin"].lastSentID("entityKey"))
	assert.Equal(t, int64(1), ds.plugins["metadata/plugin"].deltaID("entityKey"))
}

func TestUpdateLastDeltaSentNewDelta(t *testing.T) {
	s := SetUpTest(t)
	defer s.TearDownTest()
	ds := NewStore(s.repoDir, "default", maxInventorySize, true)
	s.plugin.setDeltaID("entityKey", 1)
	ds.plugins["metadata/plugin"] = s.plugin
	diff := make(map[string]interface{})
	diff["test"] = "value"
	delta := &inventoryapi.RawDelta{
		Source:    "metadata/plugin",
		ID:        int64(1),
		Timestamp: int64(1),
		Diff:      diff,
	}
	resultHint := inventoryapi.DeltaState{
		SendNextID: int64(2),
	}

	dsm := make(inventoryapi.DeltaStateMap)
	dsm[delta.Source] = &resultHint
	deltaArr := []*inventoryapi.RawDelta{delta}
	ds.UpdateState("entityKey", deltaArr, &dsm)

	assert.Equal(t, int64(1), ds.plugins["metadata/plugin"].lastSentID("entityKey"))
	assert.Equal(t, int64(1), ds.plugins["metadata/plugin"].deltaID("entityKey"))
}

func TestUpdateLastDeltaSentHintResend(t *testing.T) {
	s := SetUpTest(t)
	defer s.TearDownTest()
	ds := NewStore(s.repoDir, "default", maxInventorySize, true)
	s.plugin.setDeltaID("entityKey", 1)
	ds.plugins["metadata/plugin"] = s.plugin
	diff := make(map[string]interface{})
	diff["test"] = "value"
	delta := &inventoryapi.RawDelta{
		Source:    "metadata/plugin",
		ID:        int64(1),
		Timestamp: int64(1),
		Diff:      diff,
	}
	resultHint := inventoryapi.DeltaState{
		SendNextID: int64(0),
	}

	dsm := make(inventoryapi.DeltaStateMap)
	dsm[delta.Source] = &resultHint
	deltaArr := []*inventoryapi.RawDelta{delta}
	ds.UpdateState("entityKey", deltaArr, &dsm)

	assert.Equal(t, int64(0), ds.plugins["metadata/plugin"].lastSentID("entityKey"))
	assert.Equal(t, int64(1), ds.plugins["metadata/plugin"].deltaID("entityKey"))
}

func TestUpdateLastDeltaSentHintRequestOlder(t *testing.T) {
	s := SetUpTest(t)
	defer s.TearDownTest()
	ds := NewStore(s.repoDir, "default", maxInventorySize, true)
	s.plugin.setDeltaID("entityKey", 1)
	ds.plugins["metadata/plugin"] = s.plugin
	diff := make(map[string]interface{})
	diff["test"] = "value"
	delta := &inventoryapi.RawDelta{
		Source:    "metadata/plugin",
		ID:        int64(5),
		Timestamp: int64(1),
		Diff:      diff,
	}
	resultHint := inventoryapi.DeltaState{
		SendNextID:   int64(3),
		LastStoredID: int64(1),
	}

	dsm := make(inventoryapi.DeltaStateMap)
	dsm[delta.Source] = &resultHint
	deltaArr := []*inventoryapi.RawDelta{delta}
	ds.UpdateState("entityKey", deltaArr, &dsm)

	assert.Equal(t, int64(2), ds.plugins["metadata/plugin"].lastSentID("entityKey"))
	assert.Equal(t, int64(1), ds.plugins["metadata/plugin"].deltaID("entityKey"))
}

func TestUpdateLastDeltaSentHintIsSameAsDelta(t *testing.T) {
	s := SetUpTest(t)
	defer s.TearDownTest()
	ds := NewStore(s.repoDir, "default", maxInventorySize, true)
	s.plugin.setDeltaID("entityKey", 1)
	ds.plugins["metadata/plugin"] = s.plugin
	diff := make(map[string]interface{})
	diff["test"] = "value"
	delta := &inventoryapi.RawDelta{
		Source:    "metadata/plugin",
		ID:        int64(2),
		Timestamp: int64(1),
		Diff:      diff,
	}
	resultHint := inventoryapi.DeltaState{
		SendNextID:   int64(2),
		LastStoredID: int64(1),
	}

	dsm := make(inventoryapi.DeltaStateMap)
	dsm[delta.Source] = &resultHint
	deltaArr := []*inventoryapi.RawDelta{delta}
	ds.UpdateState("entityKey", deltaArr, &dsm)

	assert.Equal(t, int64(1), ds.plugins["metadata/plugin"].lastSentID("entityKey"))
	assert.Equal(t, int64(1), ds.plugins["metadata/plugin"].deltaID("entityKey"))
}

func TestUpdatePluginInventoryCacheFirstRunGP(t *testing.T) {
	s := SetUpTest(t)
	defer s.TearDownTest()
	const eKey = "entity:ID"

	ds := NewStore(s.repoDir, "default", maxInventorySize, true)
	srcFile := ds.SourceFilePath(s.plugin, eKey)
	err := os.MkdirAll(filepath.Dir(srcFile), 0755)
	require.NoError(t, err)
	diff := []byte(`{"hostname":{"alias":"eee-opsmatic","id":"hostname"}}`)
	err = ioutil.WriteFile(srcFile, diff, 0644)
	require.NoError(t, err)

	_, err = os.Stat(ds.cachedFilePath(s.plugin, eKey))
	assert.Error(t, err)

	updated, err := ds.updatePluginInventoryCache(s.plugin, eKey)
	require.NoError(t, err)
	assert.True(t, updated)

	assert.Equal(t, int64(0), ds.plugins["metadata/plugin"].lastSentID(eKey))
	assert.Equal(t, int64(1), ds.plugins["metadata/plugin"].deltaID(eKey))
	_, err = os.Stat(ds.cachedFilePath(s.plugin, eKey))
	require.NoError(t, err)
}

func TestUpdatePluginInventoryCacheThreeChanges(t *testing.T) {
	s := SetUpTest(t)
	defer s.TearDownTest()
	const eKey = "entity:ID"

	ds := NewStore(s.repoDir, "default", maxInventorySize, true)
	srcFile := ds.SourceFilePath(s.plugin, eKey)
	err := os.MkdirAll(filepath.Dir(srcFile), 0755)
	require.NoError(t, err)
	diff1 := []byte(`{"hostname":{"alias":"aaa-opsmatic","id":"hostname"}}`)
	err = ioutil.WriteFile(srcFile, diff1, 0644)
	require.NoError(t, err)

	updated, err := ds.updatePluginInventoryCache(s.plugin, eKey)
	require.NoError(t, err)
	assert.True(t, updated)

	diff2 := []byte(`{"hostname":{"alias":"bbb-opsmatic","id":"hostname"}}`)
	err = ioutil.WriteFile(srcFile, diff2, 0644)
	require.NoError(t, err)

	updated, err = ds.updatePluginInventoryCache(s.plugin, eKey)
	require.NoError(t, err)
	assert.True(t, updated)

	diff3 := []byte(`{"hostname":{"alias":"ccc-opsmatic","id":"hostname"}}`)
	err = ioutil.WriteFile(srcFile, diff3, 0644)
	require.NoError(t, err)

	updated, err = ds.updatePluginInventoryCache(s.plugin, eKey)
	require.NoError(t, err)
	assert.True(t, updated)

	assert.Equal(t, int64(0), ds.plugins["metadata/plugin"].lastSentID(eKey))
	assert.Equal(t, int64(3), ds.plugins["metadata/plugin"].deltaID(eKey))
}

func TestSaveState(t *testing.T) {
	s := SetUpTest(t)
	defer s.TearDownTest()
	const eKey = "entity:ID"

	ds := NewStore(s.repoDir, "default", maxInventorySize, true)
	srcFile := ds.SourceFilePath(s.plugin, eKey)
	err := os.MkdirAll(filepath.Dir(srcFile), 0755)
	require.NoError(t, err)
	diff1 := []byte(`{"hostname":{"alias":"aaa-opsmatic","id":"hostname"}}`)
	err = ioutil.WriteFile(srcFile, diff1, 0644)
	require.NoError(t, err)

	updated, err := ds.updatePluginInventoryCache(s.plugin, eKey)
	require.NoError(t, err)
	assert.True(t, updated)

	diff2 := []byte(`{"hostname":{"alias":"bbb-opsmatic","id":"hostname"}}`)
	err = ioutil.WriteFile(srcFile, diff2, 0644)
	require.NoError(t, err)

	updated, err = ds.updatePluginInventoryCache(s.plugin, eKey)
	require.NoError(t, err)
	assert.True(t, updated)

	diff3 := []byte(`{"hostname":{"alias":"ccc-opsmatic","id":"hostname"}}`)
	err = ioutil.WriteFile(srcFile, diff3, 0644)
	require.NoError(t, err)

	updated, err = ds.updatePluginInventoryCache(s.plugin, eKey)
	require.NoError(t, err)
	assert.True(t, updated)

	err = ds.SaveState()
	require.NoError(t, err)

	assert.Equal(t, int64(0), ds.plugins["metadata/plugin"].lastSentID(eKey))
	assert.Equal(t, int64(3), ds.plugins["metadata/plugin"].deltaID(eKey))

	// Read it back in, and the numbers should be the same!
	cachedDeltaPath := filepath.Join(ds.CacheDir, srcFile)
	err = ds.readPluginIDMap(cachedDeltaPath)
	require.NoError(t, err)
	assert.Equal(t, int64(0), ds.plugins["metadata/plugin"].lastSentID(eKey))
	assert.Equal(t, int64(3), ds.plugins["metadata/plugin"].deltaID(eKey))
}

// Regression test for empty cache id file handling
func TestReadPluginIDMapNoContent(t *testing.T) {
	s := SetUpTest(t)
	defer s.TearDownTest()
	const eKey = "entity:ID"

	ds := NewStore(s.repoDir, "default", maxInventorySize, true)
	srcFile := ds.SourceFilePath(s.plugin, eKey)
	err := os.MkdirAll(filepath.Dir(srcFile), 0755)
	require.NoError(t, err)
	diff1 := []byte(`{"hostname":{"alias":"aaa-opsmatic","id":"hostname"}}`)
	err = ioutil.WriteFile(srcFile, diff1, 0644)
	require.NoError(t, err)
	updated, err := ds.updatePluginInventoryCache(s.plugin, eKey)
	require.NoError(t, err)
	assert.True(t, updated)
	err = ds.SaveState()
	require.NoError(t, err)
	cachedDeltaPath := filepath.Join(ds.CacheDir, srcFile)
	err = ds.readPluginIDMap(cachedDeltaPath)
	require.NoError(t, err)
	assert.Equal(t, int64(0), ds.plugins["metadata/plugin"].lastSentID(eKey))
	assert.Equal(t, int64(1), ds.plugins["metadata/plugin"].deltaID(eKey))

	// Remove cache content and try again, should not get an error

	pluginMapPath := filepath.Join(ds.CacheDir, CACHE_ID_FILE)
	fi, err := os.Stat(pluginMapPath)
	require.NoError(t, err)
	require.Greater(t, fi.Size(), int64(0), "CACHE_ID_FILE not populated?")
	var file *os.File
	file, err = os.OpenFile(pluginMapPath, os.O_TRUNC|os.O_WRONLY, 0644)
	require.NoError(t, err)
	_ = file.Close()
	fi, err = os.Stat(pluginMapPath)
	require.NoError(t, err)
	assert.Equal(t, int64(0), fi.Size())

	err = ds.readPluginIDMap(cachedDeltaPath)
	require.NoError(t, err)
	assert.Equal(t, int64(0), ds.plugins["metadata/plugin"].lastSentID(eKey))

}

func TestReadDeltas(t *testing.T) {
	s := SetUpTest(t)
	defer s.TearDownTest()
	// Given a delta file store
	ds := NewStore(s.repoDir, "default", maxInventorySize, true)

	// When a delta source file is created for an entity
	const eKey = "entity:ID"
	srcFile := ds.SourceFilePath(s.plugin, eKey)
	err := os.MkdirAll(filepath.Dir(srcFile), 0755)
	require.NoError(t, err)
	deltaBytes := []byte(`{"hostname":{"alias":"foo","id":"hostname"}}`)
	err = ioutil.WriteFile(srcFile, deltaBytes, 0644)
	require.NoError(t, err)

	// And its cache is updated
	updated, err := ds.updatePluginInventoryCache(s.plugin, eKey)
	require.NoError(t, err)
	assert.True(t, updated)

	// Then reading deltas for the entity returns written delta
	deltas, err := ds.ReadDeltas(eKey)
	require.NoError(t, err)
	assert.Len(t, deltas, 1)
	assert.Len(t, deltas[0], 1)
	assert.Equal(t, int64(1), deltas[0][0].ID)
	assert.Equal(t, s.plugin.Source, deltas[0][0].Source)
	var expectedDelta map[string]interface{}
	require.NoError(t, json.Unmarshal(deltaBytes, &expectedDelta))
	assert.Equal(t, expectedDelta, deltas[0][0].Diff)
}

func TestReadDeltas_SamePluginWithMultipleEntitiesIncreaseIDIndependently(t *testing.T) {
	s := SetUpTest(t)
	defer s.TearDownTest()
	// Given a delta file store
	ds := NewStore(s.repoDir, "default", maxInventorySize, true)

	// When a delta source file is created for an entity
	const e1 = "entity:ID1"
	srcFile := ds.SourceFilePath(s.plugin, e1)
	err := os.MkdirAll(filepath.Dir(srcFile), 0755)
	require.NoError(t, err)
	deltaBytes := []byte(`{"hostname":{"alias":"foo","id":"hostname"}}`)
	err = ioutil.WriteFile(srcFile, deltaBytes, 0644)
	require.NoError(t, err)

	// And its cache is updated
	updated, err := ds.updatePluginInventoryCache(s.plugin, e1)
	require.NoError(t, err)
	assert.True(t, updated)

	// And read deltas for the entity 1 returns ID as 1
	deltas, err := ds.ReadDeltas(e1)
	require.NoError(t, err)
	assert.Len(t, deltas, 1)
	assert.Len(t, deltas[0], 1)
	assert.Equal(t, int64(1), deltas[0][0].ID)

	// When a delta source file is created for an entity
	const e2 = "entity:ID2"
	srcFile = ds.SourceFilePath(s.plugin, e2)
	err = os.MkdirAll(filepath.Dir(srcFile), 0755)
	require.NoError(t, err)
	deltaBytes = []byte(`{"hostname":{"alias":"bar","id":"hostname"}}`)
	err = ioutil.WriteFile(srcFile, deltaBytes, 0644)
	require.NoError(t, err)

	// And its cache is updated
	updated, err = ds.updatePluginInventoryCache(s.plugin, e2)
	require.NoError(t, err)
	assert.True(t, updated)

	// Then read deltas for the entity 2 returns ID as 1
	deltas, err = ds.ReadDeltas(e2)
	require.NoError(t, err)
	assert.Len(t, deltas, 1)
	assert.Len(t, deltas[0], 1)
	assert.Equal(t, int64(1), deltas[0][0].ID)
}

func TestReadDeltas_Divided(t *testing.T) {
	s := SetUpTest(t)
	defer s.TearDownTest()
	const eKey = "entity:ID"

	// Given some deltas
	deltas := []struct {
		source string
		diff   []byte
	}{
		{"hostname/alias", []byte(`{"hostname":{"alias":"aaa-opsmatic","id":"hostname"}}`)},
		{"something/else", []byte(`{"something":{"else":"is","being":"attached"}}`)},
		{"this/too", []byte(`{"this":{"goes":"in","another":"group"}}`)},
	}

	// And a storer whose max inventory size is lower than the sum of the 3 (each one occupies ~150 bytes)
	ds := NewStore(s.repoDir, "default", 350, true)

	var updated bool
	// And the deltas have been correctly stored
	for _, delta := range deltas {
		groupTerm := strings.Split(delta.source, "/")
		pi := PluginInfo{
			Source:   delta.source,
			Plugin:   groupTerm[0],
			FileName: groupTerm[1] + ".json",
		}
		srcFile := ds.SourceFilePath(&pi, eKey)
		err := os.MkdirAll(filepath.Dir(srcFile), 0755)
		require.NoError(t, err)

		err = ioutil.WriteFile(srcFile, delta.diff, 0644)
		require.NoError(t, err)
		updated, err = ds.updatePluginInventoryCache(&pi, eKey)
		require.NoError(t, err)
		assert.True(t, updated)
	}

	// When reading the deltas
	all, err := ds.ReadDeltas(eKey)
	require.NoError(t, err)

	// They have been read in two groups
	assert.Len(t, all, 2)
	assert.Len(t, all[0], 2)
	assert.Equal(t, "hostname/alias", all[0][0].Source)
	assert.NotNil(t, all[0][0].Diff["hostname"])
	assert.Equal(t, "something/else", all[0][1].Source)
	assert.NotNil(t, all[0][1].Diff["something"])
	assert.Len(t, all[1], 1)
	assert.Equal(t, "this/too", all[1][0].Source)
	assert.NotNil(t, all[1][0].Diff["this"])
}

func TestReadDeltas_Undivided(t *testing.T) {
	s := SetUpTest(t)
	defer s.TearDownTest()
	const eKey = "entity:ID"

	// Given some deltas
	deltas := []struct {
		source string
		diff   []byte
	}{
		{"hostname/alias", []byte(`{"hostname":{"alias":"aaa-opsmatic","id":"hostname"}}`)},
		{"something/else", []byte(`{"something":{"else":"is","being":"attached"}}`)},
		{"this/too", []byte(`{"this":{"goes":"in","another":"group"}}`)},
	}

	// And a storer whose max inventory size higher than the sum of the 3
	ds := NewStore(s.repoDir, "default", maxInventorySize, true)

	var updated bool
	// And the deltas have been correctly stored
	for _, delta := range deltas {
		groupTerm := strings.Split(delta.source, "/")
		pi := PluginInfo{
			Source:   delta.source,
			Plugin:   groupTerm[0],
			FileName: groupTerm[1] + ".json",
		}
		srcFile := ds.SourceFilePath(&pi, eKey)
		err := os.MkdirAll(filepath.Dir(srcFile), 0755)
		require.NoError(t, err)

		err = ioutil.WriteFile(srcFile, delta.diff, 0644)
		require.NoError(t, err)
		updated, err = ds.updatePluginInventoryCache(&pi, eKey)
		require.NoError(t, err)
		assert.True(t, updated)
	}

	// When reading the deltas
	all, err := ds.ReadDeltas(eKey)
	require.NoError(t, err)

	// They have been read in two groups
	// They have been read in two groups
	assert.Len(t, all, 1)
	assert.Len(t, all[0], 3)
	assert.Equal(t, "hostname/alias", all[0][0].Source)
	assert.NotNil(t, all[0][0].Diff["hostname"])
	assert.Equal(t, "something/else", all[0][1].Source)
	assert.NotNil(t, all[0][1].Diff["something"])
	assert.Equal(t, "this/too", all[0][2].Source)
	assert.NotNil(t, all[0][2].Diff["this"])
}

// COMPACTION TESTING
func (d *DeltaUtilsCoreSuite) SetupSavedState(t *testing.T) (ds *Store) {
	const eKey = "entity:ID"

	ds = NewStore(d.repoDir, "default", maxInventorySize, true)
	srcFile := ds.SourceFilePath(d.plugin, eKey)
	err := os.MkdirAll(filepath.Dir(srcFile), 0755)
	require.NoError(t, err)
	diff1 := []byte(`{"hostname":{"alias":"aaa-opsmatic","id":"hostname"}}`)
	err = ioutil.WriteFile(srcFile, diff1, 0644)
	require.NoError(t, err)

	updated, err := ds.updatePluginInventoryCache(d.plugin, eKey)
	require.NoError(t, err)
	assert.True(t, updated)

	diff2 := []byte(`{"hostname":{"alias":"bbb-opsmatic","id":"hostname"}}`)
	err = ioutil.WriteFile(srcFile, diff2, 0644)
	require.NoError(t, err)

	updated, err = ds.updatePluginInventoryCache(d.plugin, eKey)
	require.NoError(t, err)
	assert.True(t, updated)

	diff3 := []byte(`{"hostname":{"alias":"ccc-opsmatic","id":"hostname"}}`)
	err = ioutil.WriteFile(srcFile, diff3, 0644)
	require.NoError(t, err)

	updated, err = ds.updatePluginInventoryCache(d.plugin, eKey)
	require.NoError(t, err)
	assert.True(t, updated)

	err = ds.SaveState()

	require.NoError(t, err)
	assert.Equal(t, int64(0), ds.plugins["metadata/plugin"].lastSentID(eKey))
	assert.Equal(t, int64(3), ds.plugins["metadata/plugin"].deltaID(eKey))

	return ds
}

func TestCompactStoreNoChange(t *testing.T) {
	s := SetUpTest(t)
	defer s.TearDownTest()
	ds := s.SetupSavedState(t)
	size, err := ds.StorageSize(ds.CacheDir)
	require.NoError(t, err)

	err = ds.CompactStorage("", size+1024)

	require.NoError(t, err)
	newSize, err := ds.StorageSize(ds.CacheDir)
	require.NoError(t, err)
	assert.Equal(t, size, newSize)
}

func TestCompactStoreTrimSentDelta(t *testing.T) {
	const eKey = "entity:ID"
	s := SetUpTest(t)
	defer s.TearDownTest()

	ds := s.SetupSavedState(t)
	ds.plugins["metadata/plugin"].setLastSentID(eKey, 2)
	err := ds.archivePlugin(ds.plugins["metadata/plugin"], eKey)
	require.NoError(t, err)
	size, err := ds.StorageSize(ds.CacheDir)
	require.NoError(t, err)

	err = ds.CompactStorage(eKey, size-128)

	require.NoError(t, err)
	newSize, err := ds.StorageSize(ds.CacheDir)
	require.NoError(t, err)
	assert.Greater(t, size, newSize)
}

func TestCompactStoreRemoveUnusedPlugin(t *testing.T) {
	const eKey = "entity:ID"
	s := SetUpTest(t)
	defer s.TearDownTest()

	ds := s.SetupSavedState(t)
	plugin2 := newPluginInfo("fancy", "plugin.json")
	fancyFile := ds.SourceFilePath(plugin2, eKey)
	err := os.MkdirAll(filepath.Dir(fancyFile), 0755)
	require.NoError(t, err)
	diffFancy := []byte(`{"fancy":{"alias":"thing1","id":"one"}}`)
	err = ioutil.WriteFile(fancyFile, diffFancy, 0644)
	require.NoError(t, err)
	updated, err := ds.updatePluginInventoryCache(plugin2, eKey)
	require.NoError(t, err)
	assert.True(t, updated)
	_, err = os.Stat(ds.SourceFilePath(plugin2, eKey))
	require.NoError(t, err)
	_, err = os.Stat(ds.cachedFilePath(plugin2, eKey))
	require.NoError(t, err)
	_, err = os.Stat(ds.DeltaFilePath(plugin2, eKey))
	require.NoError(t, err)
	err = os.Remove(ds.SourceFilePath(plugin2, eKey))
	require.NoError(t, err)
	size, err := ds.StorageSize(ds.CacheDir)
	require.NoError(t, err)

	err = ds.CompactStorage(eKey, size-128)
	require.NoError(t, err)

	newSize, err := ds.StorageSize(ds.CacheDir)
	require.NoError(t, err)
	assert.Greater(t, size, newSize)
	_, err = os.Stat(ds.SourceFilePath(plugin2, eKey))
	assert.Error(t, err)
	_, err = os.Stat(ds.cachedFilePath(plugin2, eKey))
	assert.Error(t, err)
	_, err = os.Stat(ds.DeltaFilePath(plugin2, eKey))
	assert.Error(t, err)
}

func TestStoreNotArchiving(t *testing.T) {
	const eKey = "entity:ID"
	s := SetUpTest(t)
	defer s.TearDownTest()

	ds := s.SetupSavedState(t)
	ds.archiveEnabled = false
	ds.plugins["metadata/plugin"].setLastSentID(eKey, 2)
	origSize, err := ds.StorageSize(ds.CacheDir)
	require.NoError(t, err)

	err = ds.archivePlugin(ds.plugins["metadata/plugin"], eKey)
	require.NoError(t, err)

	require.NoError(t, err)
	newSize, err := ds.StorageSize(ds.CacheDir)
	require.NoError(t, err)
	assert.Less(t, newSize, origSize)

	exists := exists(filepath.Join(ds.CacheDir, "metadata/entityID/plugin.sent"))
	assert.False(t, exists, "expected .sent file to not exist")
}

func TestDeltaFileCorrupt(t *testing.T) {
	s := SetUpTest(t)
	defer s.TearDownTest()
	const eKey = "entity:ID"
	ds := NewStore(s.repoDir, "default", maxInventorySize, true)
	srcFile := ds.SourceFilePath(s.plugin, eKey)
	err := os.MkdirAll(filepath.Dir(srcFile), 0755)
	require.NoError(t, err)
	diff1 := []byte(`{"hostname":{"alias":"aaa-opsmatic","id":"hostname"}}`)
	err = ioutil.WriteFile(srcFile, diff1, 0644)
	require.NoError(t, err)

	updated, err := ds.updatePluginInventoryCache(s.plugin, eKey)
	require.NoError(t, err)
	assert.True(t, updated)

	corruptDeltaFile := ds.DeltaFilePath(s.plugin, eKey)
	err = os.MkdirAll(filepath.Dir(corruptDeltaFile), 0755)
	require.NoError(t, err)
	corruptDelta := []byte(`{"source":"test/thing","id":1,`)
	err = ioutil.WriteFile(corruptDeltaFile, corruptDelta, 0644)
	require.NoError(t, err)

	secondPlugin := newPluginInfo("metadata", "plugin.json")
	// break on purpose, so read should fail
	secondPlugin.Source = "metadata/plugin2"
	secondPlugin.Plugin = "metadata2"

	srcFile2 := ds.SourceFilePath(secondPlugin, eKey)
	err = os.MkdirAll(filepath.Dir(srcFile2), 0755)
	require.NoError(t, err)
	diff2 := []byte(`{"hostname":{"alias":"bbb-opsmatic","id":"ipAddress"}}`)
	err = ioutil.WriteFile(srcFile2, diff2, 0644)
	require.NoError(t, err)

	updated, err = ds.updatePluginInventoryCache(secondPlugin, eKey)
	require.NoError(t, err)
	assert.True(t, updated)

	normalDeltaFile := ds.DeltaFilePath(secondPlugin, eKey)
	err = os.MkdirAll(filepath.Dir(normalDeltaFile), 0755)
	require.NoError(t, err)
	normalDelta := []byte(`{"source":"test/thing2","id":1,"timestamp":1487182523,"diff":null,"full_diff":false}`)

	err = ioutil.WriteFile(normalDeltaFile, normalDelta, 0644)
	require.NoError(t, err)

	_, err = ds.ReadDeltas(eKey)
	assert.Error(t, err)

	_, err = ds.ReadDeltas(eKey)
	require.NoError(t, err)

	data, err := ioutil.ReadFile(corruptDeltaFile)
	require.NoError(t, err)
	assert.Empty(t, data)

	data, err = ioutil.ReadFile(normalDeltaFile)
	require.NoError(t, err)
	actual := string(data)
	assert.Equal(t, `{"source":"test/thing2","id":1,"timestamp":1487182523,"diff":null,"full_diff":false}`, actual)
}

func TestRemoveEntity(t *testing.T) {
	const aPlugin = "aPlugin"
	const anotherPlugin = "anotherPlugin"
	const entityToKeep = "entityToKeep"
	const entityToRemove = "entityToRemove"

	// Given a Store object under a base data directory
	baseDir, err := ioutil.TempDir("", "test-remove-entity")
	require.NoError(t, err)
	defer os.RemoveAll(baseDir)

	directories := []struct {
		path            string
		shouldBeRemoved bool
	}{
		{filepath.Join(baseDir, aPlugin, entityToKeep), false},
		{filepath.Join(baseDir, aPlugin, entityToRemove), true},
		{filepath.Join(baseDir, anotherPlugin, entityToKeep), false},
		{filepath.Join(baseDir, CACHE_DIR, aPlugin, entityToKeep), false},
		{filepath.Join(baseDir, CACHE_DIR, anotherPlugin, entityToKeep), false},
		{filepath.Join(baseDir, CACHE_DIR, anotherPlugin, entityToRemove), true},
		{filepath.Join(baseDir, SAMPLING_REPO, "ignoreThis", "ignoreThis"), false},
	}
	for _, dir := range directories {
		assert.NoError(t, os.MkdirAll(dir.path, 0755))
	}
	store := NewStore(baseDir, "default", maxInventorySize, true)

	// When removing data from a given entity:
	_ = store.RemoveEntityFolders(entityToRemove)

	// Then only the folders from such entity are removed
	for _, dir := range directories {
		_, err := os.Stat(dir.path)
		if dir.shouldBeRemoved {
			assert.True(t, os.IsNotExist(err))
		} else {
			require.NoError(t, err)
		}
	}
}

func TestScanEntityFolders(t *testing.T) {
	const aPlugin = "aPlugin"
	const anotherPlugin = "anotherPlugin"
	const anEntity = "anEntity"
	const anotherEntity = "anotherEntity"

	// Given a Store object under a base data directory
	baseDir, err := ioutil.TempDir("", "test-scan-entity-folders")
	require.NoError(t, err)
	defer os.RemoveAll(baseDir)

	directories := []struct {
		path            string
		shouldBeRemoved bool
	}{
		{filepath.Join(baseDir, aPlugin, anEntity), false},
		{filepath.Join(baseDir, aPlugin, anotherEntity), true},
		{filepath.Join(baseDir, anotherPlugin, anEntity), false},
		{filepath.Join(baseDir, CACHE_DIR, aPlugin, anEntity), false},
		{filepath.Join(baseDir, CACHE_DIR, anotherPlugin, anEntity), false},
		{filepath.Join(baseDir, CACHE_DIR, anotherPlugin, anotherEntity), true},
		{filepath.Join(baseDir, SAMPLING_REPO, "ignoreThis", "ignoreThis"), false},
	}
	for _, dir := range directories {
		assert.NoError(t, os.MkdirAll(dir.path, 0755))
	}
	store := NewStore(baseDir, "default", maxInventorySize, true)

	// When fetching all the entities
	entities, err := store.ScanEntityFolders()
	require.NoError(t, err)

	// Then only the entities in the system are retrieved, ignoring those from cache and sample dirs
	assert.Equal(t, map[string]interface{}{"anEntity": true, "anotherEntity": true}, entities)
}

func TestCollectPluginFiles(t *testing.T) {
	const aPlugin = "aPlugin"
	const anotherPlugin = "anotherPlugin"
	const anEntity = "anEntity"
	const anotherEntity = "anotherEntity"

	// Given a Store object under a base data directory
	baseDir, err := ioutil.TempDir("", "test-scan-entity-folders")
	require.NoError(t, err)
	defer os.RemoveAll(baseDir)

	directories := []struct {
		path            string
		shouldBeRemoved bool
	}{
		{filepath.Join(baseDir, aPlugin, anEntity, "aFile.json"), false},
		{filepath.Join(baseDir, aPlugin, anotherEntity, "aFile.json"), true},
		{filepath.Join(baseDir, anotherPlugin, anEntity, "anotherFile.json"), false},
		{filepath.Join(baseDir, CACHE_DIR, aPlugin, anEntity, "aFile.json"), false},
		{filepath.Join(baseDir, CACHE_DIR, anotherPlugin, anEntity, "anotherFile.json"), false},
		{filepath.Join(baseDir, CACHE_DIR, anotherPlugin, anotherEntity, "anotherFile.json"), true},
		{filepath.Join(baseDir, SAMPLING_REPO, "ignoreThis", "ignoreThis", "ignore.json"), false},
	}
	for _, dir := range directories {
		require.NoError(t, os.MkdirAll(filepath.Dir(dir.path), 0755))
		file, err := os.Create(dir.path)
		require.NoError(t, err)
		file.Close()
	}
	store := NewStore(baseDir, "default", maxInventorySize, true)

	// When collecting all the plugins of a given entity
	plugins, err := store.collectPluginFiles(store.DataDir, anEntity, helpers.JsonFilesRegexp)
	require.NoError(t, err)
	// They got the expected values
	assert.Len(t, plugins, 2)
	assert.NotEqual(t, plugins[0].Plugin, plugins[1].Plugin)
	expectedPlugins := map[string]bool{"aPlugin/aFile": true, "anotherPlugin/anotherFile": true}
	assert.True(t, expectedPlugins[plugins[0].Source])
	assert.True(t, expectedPlugins[plugins[1].Source])

	// When collecting other plugins for the given entity
	plugins, err = store.collectPluginFiles(store.DataDir, anotherEntity, helpers.JsonFilesRegexp)
	require.NoError(t, err)
	assert.NotEmpty(t, plugins)
	// They got the correct ones
	for _, plugin := range plugins {
		assert.Equal(t, "aPlugin/aFile", plugin.Source)
	}
}

func TestUpdatePluginInventoryCacheDeltaFileCorrupted(t *testing.T) {
	testCases := []map[string][]byte{{
		"source": []byte(`{"hostname":{"alias":"aaa-opsmatic","id":"hostname"`),
		"cache":  []byte(`{"hostname":{"alias":"aaa-opsmatic","id":"hostname"}}`),
	}, {
		"source": []byte(`{"hostname":{"alias":"aaa-opsmatic","id":"hostname"}}`),
		"cache":  []byte(`{"hostname":{"alias":"aaa-opsmatic","id":"hostname"`),
	}}
	dataDir, err := TempDeltaStoreDir()
	require.NoError(t, err)
	defer os.RemoveAll(dataDir)

	for _, testCase := range testCases {
		// Given corrupted data and deltas
		sourceDir := filepath.Join(dataDir, "corrupted", localEntityFolder)
		sourceJSON := filepath.Join(sourceDir, "corrupted.json")
		cacheDir := filepath.Join(dataDir, CACHE_DIR, "corrupted", localEntityFolder)
		cacheJSON := filepath.Join(cacheDir, "corrupted.json")

		// And a delta storage
		ds := NewStore(dataDir, "default", maxInventorySize, true)
		require.NoError(t, os.MkdirAll(sourceDir, 0755))
		require.NoError(t, os.MkdirAll(cacheDir, 0755))
		require.NoError(t, ioutil.WriteFile(sourceJSON, testCase["source"], 0644))
		require.NoError(t, ioutil.WriteFile(cacheJSON, testCase["cache"], 0644))

		// When the updatePluginInventoryCache method tries to deal with the corrupted JSONs
		updated, err := ds.updatePluginInventoryCache(&PluginInfo{
			Source:   "corrupted/corrupted",
			Plugin:   "corrupted",
			FileName: "corrupted.json",
		}, "")

		assert.True(t, updated)
		assert.Error(t, err)

		// The corrupted plugin is removed
		_, err = os.Stat(cacheJSON)
		assert.True(t, os.IsNotExist(err))
		_, err = os.Stat(sourceJSON)
		assert.True(t, os.IsNotExist(err))
	}
}

func TestDeltaRoot_WithCorruptedFile_StartFresh(t *testing.T) {
	// GIVEN an empty folder for the inventory data dir
	dataPath := filepath.Join(os.TempDir(), "WithCorruptedFile_StartFresh")
	err := os.MkdirAll(dataPath, DATA_DIR_MODE)
	assert.NoError(t, err)

	// GIVEN an empty folder for the inventory cache dir
	cacheDir := filepath.Join(dataPath, CACHE_DIR)
	err = os.MkdirAll(cacheDir, DATA_DIR_MODE)
	assert.NoError(t, err)

	// GIVEN a corrupted json file located in the cache directory
	jsonFile := filepath.Join(cacheDir, CACHE_ID_FILE)
	err = ioutil.WriteFile(jsonFile, []byte(`{"inventory":{"source":"name",`), DATA_FILE_MODE)
	assert.NoError(t, err)

	// WHEN the data store is create
	ds := NewStore(dataPath, "default", maxInventorySize, true)
	assert.NotNil(t, ds)

	// THEN check that the corrupted json file has been deleted
	_, err = os.Stat(jsonFile)
	assert.Error(t, err, "Corrupted delta root json file wasn't removed")
}

func TestStore_Path_Exists(t *testing.T) {
	// GIVEN an existing path
	path := "./"

	// WHEN check if the provided path exists
	exists := exists(path)

	// THEN should return that exists as a bool
	assert.True(t, exists,
		fmt.Sprintf("Expected that the path: '%s' exists, but the function return: %t", path, exists))
}

func TestStore_Path_NotExists(t *testing.T) {
	// GIVEN a non existing path
	path := "someNonExistingPath"

	// WHEN check if the provided path exists
	exists := exists(path)

	// THEN should return that not exists as a bool
	assert.False(t, exists,
		fmt.Sprintf("Expected that the path: '%s' not exists, but the function return: %t", path, exists))
}

func TestStore_Path_PermissionDenied(t *testing.T) {
	// GIVEN an existing path with no permissions
	dataPath := filepath.Join(os.TempDir(), "Path_PermissionDenied")
	const NO_PERM = os.FileMode(0000)
	err := os.MkdirAll(dataPath, NO_PERM)
	assert.NoError(t, err)

	// WHEN check if the provided path exists
	exists := exists(dataPath)

	// THEN should return that exists as a bool
	assert.True(t, exists)
}
