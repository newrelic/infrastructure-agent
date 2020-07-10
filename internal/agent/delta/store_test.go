// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package delta

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"

	"github.com/newrelic/infrastructure-agent/pkg/backend/inventoryapi"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"

	. "github.com/newrelic/infrastructure-agent/pkg/go-better-check"
	. "gopkg.in/check.v1"
)

const maxInventorySize = 3 * 1000 * 1000

func Test(t *testing.T) { TestingT(t) }

func TempDeltaStoreDir() (string, error) {
	return ioutil.TempDir("", "deltastore")
}

type DeltaUtilsSuite struct {
}

var _ = Suite(&DeltaUtilsSuite{})

func (s *DeltaUtilsSuite) TestRemoveNulls(c *C) {
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
	c.Assert(hasChild1Attr3, Equals, false)

	child2 := obj["child2"].([]interface{})
	child2Map := child2[2].(map[string]interface{})
	_, hasChild2MapName := child2Map["name"]
	c.Assert(hasChild2MapName, Equals, false)

	_, hasChild3 := obj["child3"]
	c.Assert(hasChild3, Equals, false)
}

func (s *DeltaUtilsSuite) TestPluginInfoNextDelta(c *C) {
	pi := &PluginInfo{}

	c.Assert(pi.MostRecentID, Equals, int64(0))
	c.Assert(pi.nextDeltaID(), Equals, int64(1))
	c.Assert(pi.MostRecentID, Equals, int64(1))
	pi.MostRecentID = int64(99)
	c.Assert(pi.nextDeltaID(), Equals, int64(100))
}

func (s *DeltaUtilsSuite) TestNewDeltaStoreGolden(c *C) {
	dataDir, err := TempDeltaStoreDir()
	c.Check(err, IsNil)
	repoDir := filepath.Join(dataDir, "delta")
	ds := NewStore(repoDir, "default", maxInventorySize)

	c.Assert(ds, Not(IsNil))

	syscall.Rmdir(dataDir)
}

func (s *DeltaUtilsSuite) TestStorageSize(c *C) {
	dataDir, err := TempDeltaStoreDir()
	c.Check(err, IsNil)
	repoDir := filepath.Join(dataDir, "delta")
	ds := NewStore(repoDir, "default", maxInventorySize)

	size, _ := ds.StorageSize(ds.CacheDir)
	c.Assert(size, Equals, uint64(0))

	deltaFilePath := filepath.Join(ds.CacheDir, "test")
	f, err := os.OpenFile(deltaFilePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	c.Check(err, IsNil)
	defer f.Close()
	buf := []byte{'1', 'A', '#'}
	_, err = f.Write(buf)
	c.Check(err, IsNil)

	size, _ = ds.StorageSize(ds.CacheDir)
	c.Assert(size, Equals, uint64(len(buf)))

	syscall.Rmdir(dataDir)
}

type DeltaUtilsCoreSuite struct {
	dataDir  string
	repoDir  string
	plugin   *PluginInfo
	pluginID ids.PluginID
}

var _core = Suite(&DeltaUtilsCoreSuite{})

func (s *DeltaUtilsCoreSuite) SetUpTest(c *C) {
	var err error
	s.dataDir, err = TempDeltaStoreDir()
	c.Check(err, IsNil)
	s.repoDir = filepath.Join(s.dataDir, "delta")
	s.plugin = &PluginInfo{
		Source:       "metadata/plugin",
		Plugin:       "metadata",
		FileName:     "plugin.json",
		MostRecentID: int64(0),
		LastSentID:   int64(0),
	}
	s.pluginID = ids.PluginID{
		Category: "metadata",
		Term:     "plugin",
	}
}

func (s *DeltaUtilsCoreSuite) TearDownTest(c *C) {
	os.RemoveAll(s.dataDir)
}

func (s *DeltaUtilsCoreSuite) TestArchiveFilePath(c *C) {
	ds := NewStore(s.repoDir, "default", maxInventorySize)
	c.Assert(ds.archiveFilePath(s.plugin, "entity:id"), HasSuffix, filepath.Join("delta", ".delta_repo", "metadata", "entityid", "plugin.sent"))
}

func (s *DeltaUtilsCoreSuite) TestDeltaFilePath(c *C) {
	ds := NewStore(s.repoDir, "default", maxInventorySize)

	c.Assert(ds.DeltaFilePath(s.plugin, "entity:id:2"), HasSuffix, filepath.Join("delta", ".delta_repo", "metadata", "entityid2", "plugin.pending"))
}

func (s *DeltaUtilsCoreSuite) TestCachedFilePath(c *C) {
	ds := NewStore(s.repoDir, "default", maxInventorySize)

	c.Assert(ds.cachedFilePath(s.plugin, "hello!!everybody"), HasSuffix, filepath.Join("delta", ".delta_repo", "metadata", "hello!!everybody", "plugin.json"))
}

func (s *DeltaUtilsCoreSuite) TestSourceFilePath(c *C) {
	ds := NewStore(s.repoDir, "default", maxInventorySize)

	c.Assert(ds.SourceFilePath(s.plugin, "xxxx"), HasSuffix, filepath.Join("delta", "metadata", "xxxx", "plugin.json"))
}

func (s *DeltaUtilsCoreSuite) TestArchiveFilePath_localEntity(c *C) {
	ds := NewStore(s.repoDir, "my-hostname", maxInventorySize)
	c.Assert(ds.archiveFilePath(s.plugin, ""), HasSuffix, filepath.Join("delta", ".delta_repo", "metadata", localEntityFolder, "plugin.sent"))
}

func (s *DeltaUtilsCoreSuite) TestDeltaFilePath_localEntity(c *C) {
	ds := NewStore(s.repoDir, "my-hostname", maxInventorySize)

	c.Assert(ds.DeltaFilePath(s.plugin, ""), HasSuffix, filepath.Join("delta", ".delta_repo", "metadata", localEntityFolder, "plugin.pending"))
}

func (s *DeltaUtilsCoreSuite) TestCachedFilePath_localEntity(c *C) {
	ds := NewStore(s.repoDir, "my-hostname", maxInventorySize)

	c.Assert(ds.cachedFilePath(s.plugin, ""), HasSuffix, filepath.Join("delta", ".delta_repo", "metadata", localEntityFolder, "plugin.json"))
}

func (s *DeltaUtilsCoreSuite) TestSourceFilePath_localEntity(c *C) {
	ds := NewStore(s.repoDir, "my-hostname", maxInventorySize)

	c.Assert(ds.SourceFilePath(s.plugin, ""), HasSuffix, filepath.Join("delta", "metadata", localEntityFolder, "plugin.json"))
}

func (s *DeltaUtilsCoreSuite) TestBaseDirectories(c *C) {
	ds := NewStore(s.repoDir, "default", maxInventorySize)

	c.Assert(ds.DataDir, Equals, s.repoDir)
	c.Assert(ds.CacheDir, Equals, filepath.Join(s.repoDir, CACHE_DIR))
}

func (s *DeltaUtilsCoreSuite) TestResetAllSentDeltas(c *C) {
	const id = "entityKey"
	ds := NewStore(s.repoDir, "default", maxInventorySize)
	srcFile := ds.SourceFilePath(s.plugin, id)
	err := os.MkdirAll(filepath.Dir(srcFile), 0755)
	c.Assert(err, IsNil)
	diff := []byte(`{"hostname":{"alias":"eee-opsmatic","id":"hostname"}}`)
	err = ioutil.WriteFile(srcFile, diff, 0644)
	c.Assert(err, IsNil)
	_, err = os.Stat(ds.cachedFilePath(s.plugin, id))
	c.Assert(err, Not(IsNil))
	updated, err := ds.updatePluginInventoryCache(s.plugin, id)
	c.Assert(updated, Equals, true)
	c.Assert(err, IsNil)
	c.Assert(ds.NextIDMap["metadata/plugin"].LastSentID, Equals, int64(0))
	c.Assert(ds.NextIDMap["metadata/plugin"].MostRecentID, Equals, int64(1))
	_, err = os.Stat(ds.cachedFilePath(s.plugin, id))
	c.Assert(err, IsNil)

	ds.ResetAllDeltas(id)

	_, err = os.Stat(ds.cachedFilePath(s.plugin, id))
	c.Assert(err, NotNil)
}

func (s *DeltaUtilsCoreSuite) TestUpdateLastDeltaSentNoHint(c *C) {
	ds := NewStore(s.repoDir, "default", maxInventorySize)
	s.plugin.MostRecentID = int64(1)
	s.plugin.LastSentID = int64(0)
	ds.NextIDMap["metadata/plugin"] = s.plugin
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

	c.Assert(ds.NextIDMap["metadata/plugin"].LastSentID, Equals, int64(2))
	c.Assert(ds.NextIDMap["metadata/plugin"].MostRecentID, Equals, int64(1))
}

func (s *DeltaUtilsCoreSuite) TestUpdateLastDeltaSentNewDelta(c *C) {
	ds := NewStore(s.repoDir, "default", maxInventorySize)
	s.plugin.MostRecentID = int64(1)
	s.plugin.LastSentID = int64(0)
	ds.NextIDMap["metadata/plugin"] = s.plugin
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

	c.Assert(ds.NextIDMap["metadata/plugin"].LastSentID, Equals, int64(1))
	c.Assert(ds.NextIDMap["metadata/plugin"].MostRecentID, Equals, int64(1))
}

func (s *DeltaUtilsCoreSuite) TestUpdateLastDeltaSentHintResend(c *C) {
	ds := NewStore(s.repoDir, "default", maxInventorySize)
	s.plugin.MostRecentID = int64(1)
	s.plugin.LastSentID = int64(0)
	ds.NextIDMap["metadata/plugin"] = s.plugin
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

	c.Assert(ds.NextIDMap["metadata/plugin"].LastSentID, Equals, int64(0))
	c.Assert(ds.NextIDMap["metadata/plugin"].MostRecentID, Equals, int64(1))
}

func (s *DeltaUtilsCoreSuite) TestUpdateLastDeltaSentHintRequestOlder(c *C) {
	ds := NewStore(s.repoDir, "default", maxInventorySize)
	s.plugin.MostRecentID = int64(1)
	s.plugin.LastSentID = int64(0)
	ds.NextIDMap["metadata/plugin"] = s.plugin
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

	c.Assert(ds.NextIDMap["metadata/plugin"].LastSentID, Equals, int64(2))
	c.Assert(ds.NextIDMap["metadata/plugin"].MostRecentID, Equals, int64(1))
}

func (s *DeltaUtilsCoreSuite) TestUpdateLastDeltaSentHintIsSameAsDelta(c *C) {
	ds := NewStore(s.repoDir, "default", maxInventorySize)
	s.plugin.MostRecentID = int64(1)
	s.plugin.LastSentID = int64(0)
	ds.NextIDMap["metadata/plugin"] = s.plugin
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

	c.Assert(ds.NextIDMap["metadata/plugin"].LastSentID, Equals, int64(1))
	c.Assert(ds.NextIDMap["metadata/plugin"].MostRecentID, Equals, int64(1))
}

func (s *DeltaUtilsCoreSuite) TestUpdatePluginInventoryCacheFirstRunGP(c *C) {
	const id = "entity:ID"

	ds := NewStore(s.repoDir, "default", maxInventorySize)
	srcFile := ds.SourceFilePath(s.plugin, id)
	err := os.MkdirAll(filepath.Dir(srcFile), 0755)
	c.Assert(err, IsNil)
	diff := []byte(`{"hostname":{"alias":"eee-opsmatic","id":"hostname"}}`)
	err = ioutil.WriteFile(srcFile, diff, 0644)
	c.Assert(err, IsNil)

	_, err = os.Stat(ds.cachedFilePath(s.plugin, id))
	c.Assert(err, Not(IsNil))

	updated, err := ds.updatePluginInventoryCache(s.plugin, id)
	c.Assert(updated, Equals, true)

	c.Assert(err, IsNil)
	c.Assert(ds.NextIDMap["metadata/plugin"].LastSentID, Equals, int64(0))
	c.Assert(ds.NextIDMap["metadata/plugin"].MostRecentID, Equals, int64(1))
	_, err = os.Stat(ds.cachedFilePath(s.plugin, id))
	c.Assert(err, IsNil)
}

func (s *DeltaUtilsCoreSuite) TestUpdatePluginInventoryCacheThreeChanges(c *C) {
	const id = "entity:ID"

	ds := NewStore(s.repoDir, "default", maxInventorySize)
	srcFile := ds.SourceFilePath(s.plugin, id)
	err := os.MkdirAll(filepath.Dir(srcFile), 0755)
	c.Assert(err, IsNil)
	diff1 := []byte(`{"hostname":{"alias":"aaa-opsmatic","id":"hostname"}}`)
	err = ioutil.WriteFile(srcFile, diff1, 0644)
	c.Assert(err, IsNil)

	updated, err := ds.updatePluginInventoryCache(s.plugin, id)
	c.Assert(updated, Equals, true)
	c.Assert(err, IsNil)

	diff2 := []byte(`{"hostname":{"alias":"bbb-opsmatic","id":"hostname"}}`)
	err = ioutil.WriteFile(srcFile, diff2, 0644)
	c.Assert(err, IsNil)

	updated, err = ds.updatePluginInventoryCache(s.plugin, id)
	c.Assert(updated, Equals, true)
	c.Assert(err, IsNil)

	diff3 := []byte(`{"hostname":{"alias":"ccc-opsmatic","id":"hostname"}}`)
	err = ioutil.WriteFile(srcFile, diff3, 0644)
	c.Assert(err, IsNil)

	updated, err = ds.updatePluginInventoryCache(s.plugin, id)
	c.Assert(updated, Equals, true)

	c.Assert(err, IsNil)
	c.Assert(ds.NextIDMap["metadata/plugin"].LastSentID, Equals, int64(0))
	c.Assert(ds.NextIDMap["metadata/plugin"].MostRecentID, Equals, int64(3))
}

func (s *DeltaUtilsCoreSuite) TestSaveState(c *C) {
	const id = "entity:ID"

	ds := NewStore(s.repoDir, "default", maxInventorySize)
	srcFile := ds.SourceFilePath(s.plugin, id)
	err := os.MkdirAll(filepath.Dir(srcFile), 0755)
	c.Assert(err, IsNil)
	diff1 := []byte(`{"hostname":{"alias":"aaa-opsmatic","id":"hostname"}}`)
	err = ioutil.WriteFile(srcFile, diff1, 0644)
	c.Assert(err, IsNil)

	updated, err := ds.updatePluginInventoryCache(s.plugin, id)
	c.Assert(updated, Equals, true)
	c.Assert(err, IsNil)

	diff2 := []byte(`{"hostname":{"alias":"bbb-opsmatic","id":"hostname"}}`)
	err = ioutil.WriteFile(srcFile, diff2, 0644)
	c.Assert(err, IsNil)

	updated, err = ds.updatePluginInventoryCache(s.plugin, id)
	c.Assert(updated, Equals, true)
	c.Assert(err, IsNil)

	diff3 := []byte(`{"hostname":{"alias":"ccc-opsmatic","id":"hostname"}}`)
	err = ioutil.WriteFile(srcFile, diff3, 0644)
	c.Assert(err, IsNil)

	updated, err = ds.updatePluginInventoryCache(s.plugin, id)
	c.Assert(updated, Equals, true)
	c.Assert(err, IsNil)

	err = ds.SaveState()

	c.Assert(err, IsNil)
	c.Assert(ds.NextIDMap["metadata/plugin"].LastSentID, Equals, int64(0))
	c.Assert(ds.NextIDMap["metadata/plugin"].MostRecentID, Equals, int64(3))

	// Read it back in, and the numbers should be the same!
	cachedDeltaPath := filepath.Join(ds.CacheDir, srcFile)
	err = ds.readPluginIDMap(cachedDeltaPath)
	c.Assert(err, IsNil)
	c.Assert(ds.NextIDMap["metadata/plugin"].LastSentID, Equals, int64(0))
	c.Assert(ds.NextIDMap["metadata/plugin"].MostRecentID, Equals, int64(3))
}

// Regression test for empty cache id file handling
func (s *DeltaUtilsCoreSuite) TestReadPluginIDMapNoContent(c *C) {
	const id = "entity:ID"

	ds := NewStore(s.repoDir, "default", maxInventorySize)
	srcFile := ds.SourceFilePath(s.plugin, id)
	err := os.MkdirAll(filepath.Dir(srcFile), 0755)
	c.Assert(err, IsNil)
	diff1 := []byte(`{"hostname":{"alias":"aaa-opsmatic","id":"hostname"}}`)
	err = ioutil.WriteFile(srcFile, diff1, 0644)
	c.Assert(err, IsNil)
	updated, err := ds.updatePluginInventoryCache(s.plugin, id)
	c.Assert(updated, Equals, true)
	c.Assert(err, IsNil)
	err = ds.SaveState()
	c.Assert(err, IsNil)
	cachedDeltaPath := filepath.Join(ds.CacheDir, srcFile)
	err = ds.readPluginIDMap(cachedDeltaPath)
	c.Assert(err, IsNil)
	c.Assert(ds.NextIDMap["metadata/plugin"].LastSentID, Equals, int64(0))
	c.Assert(ds.NextIDMap["metadata/plugin"].MostRecentID, Equals, int64(1))

	// Remove cache content and try again, should not get an error

	pluginMapPath := filepath.Join(ds.CacheDir, CACHE_ID_FILE)
	fi, err := os.Stat(pluginMapPath)
	c.Assert(err, IsNil)
	if fi.Size() > 0 {
		var file *os.File
		file, err = os.OpenFile(pluginMapPath, os.O_TRUNC|os.O_WRONLY, 0644)
		c.Assert(err, IsNil)
		_ = file.Close()
		fi, err = os.Stat(pluginMapPath)
		c.Assert(fi.Size(), Equals, int64(0))
		c.Assert(err, IsNil)
	} else {
		c.Errorf("CACHE_ID_FILE not populated?")
	}

	err = ds.readPluginIDMap(cachedDeltaPath)
	c.Assert(err, IsNil)
	c.Assert(ds.NextIDMap["metadata/plugin"].LastSentID, Equals, int64(0))

}

func (s *DeltaUtilsCoreSuite) TestReadDeltas(c *C) {
	const id = "entity:ID"

	ds := NewStore(s.repoDir, "default", maxInventorySize)
	srcFile := ds.SourceFilePath(s.plugin, id)
	err := os.MkdirAll(filepath.Dir(srcFile), 0755)
	c.Assert(err, IsNil)
	diff1 := []byte(`{"hostname":{"alias":"aaa-opsmatic","id":"hostname"}}`)
	err = ioutil.WriteFile(srcFile, diff1, 0644)
	c.Assert(err, IsNil)
	updated, err := ds.updatePluginInventoryCache(s.plugin, id)
	c.Assert(updated, Equals, true)
	c.Assert(err, IsNil)

	deltas, err := ds.ReadDeltas(id)
	c.Assert(err, IsNil)
	c.Assert(deltas, HasLen, 1)
	c.Assert(deltas[0], HasLen, 1)
	c.Assert(deltas[0][0].Source, Equals, s.plugin.Source)
}

func (s *DeltaUtilsCoreSuite) TestReadDeltas_Divided(c *C) {
	const entityKey = "entity:ID"

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
	ds := NewStore(s.repoDir, "default", 350)

	var updated bool
	// And the deltas have been correctly stored
	for _, delta := range deltas {
		groupTerm := strings.Split(delta.source, "/")
		pi := PluginInfo{
			Source:   delta.source,
			Plugin:   groupTerm[0],
			FileName: groupTerm[1] + ".json",
		}
		srcFile := ds.SourceFilePath(&pi, entityKey)
		err := os.MkdirAll(filepath.Dir(srcFile), 0755)
		c.Assert(err, IsNil)

		err = ioutil.WriteFile(srcFile, delta.diff, 0644)
		c.Assert(err, IsNil)
		updated, err = ds.updatePluginInventoryCache(&pi, entityKey)
		c.Assert(updated, Equals, true)
		c.Assert(err, IsNil)
	}

	// When reading the deltas
	all, err := ds.ReadDeltas(entityKey)
	c.Assert(err, IsNil)

	// They have been read in two groups
	c.Assert(all, HasLen, 2)
	c.Assert(all[0], HasLen, 2)
	c.Assert(all[0][0].Source, Equals, "hostname/alias")
	c.Assert(all[0][0].Diff["hostname"], NotNil)
	c.Assert(all[0][1].Source, Equals, "something/else")
	c.Assert(all[0][1].Diff["something"], NotNil)
	c.Assert(all[1], HasLen, 1)
	c.Assert(all[1][0].Source, Equals, "this/too")
	c.Assert(all[1][0].Diff["this"], NotNil)
}

func (s *DeltaUtilsCoreSuite) TestReadDeltas_Undivided(c *C) {
	const entityKey = "entity:ID"

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
	ds := NewStore(s.repoDir, "default", maxInventorySize)

	var updated bool
	// And the deltas have been correctly stored
	for _, delta := range deltas {
		groupTerm := strings.Split(delta.source, "/")
		pi := PluginInfo{
			Source:   delta.source,
			Plugin:   groupTerm[0],
			FileName: groupTerm[1] + ".json",
		}
		srcFile := ds.SourceFilePath(&pi, entityKey)
		err := os.MkdirAll(filepath.Dir(srcFile), 0755)
		c.Assert(err, IsNil)

		err = ioutil.WriteFile(srcFile, delta.diff, 0644)
		c.Assert(err, IsNil)
		updated, err = ds.updatePluginInventoryCache(&pi, entityKey)
		c.Assert(updated, Equals, true)
		c.Assert(err, IsNil)
	}

	// When reading the deltas
	all, err := ds.ReadDeltas(entityKey)
	c.Assert(err, IsNil)

	// They have been read in two groups
	c.Assert(all, HasLen, 1)
	c.Assert(all[0], HasLen, 3)
	c.Assert(all[0][0].Source, Equals, "hostname/alias")
	c.Assert(all[0][0].Diff["hostname"], NotNil)
	c.Assert(all[0][1].Source, Equals, "something/else")
	c.Assert(all[0][1].Diff["something"], NotNil)
	c.Assert(all[0][2].Source, Equals, "this/too")
	c.Assert(all[0][2].Diff["this"], NotNil)
}

// COMPACTION TESTING
func (s *DeltaUtilsCoreSuite) SetupSavedState(c *C) (ds *Store) {
	const id = "entity:ID"

	ds = NewStore(s.repoDir, "default", maxInventorySize)
	srcFile := ds.SourceFilePath(s.plugin, id)
	err := os.MkdirAll(filepath.Dir(srcFile), 0755)
	c.Assert(err, IsNil)
	diff1 := []byte(`{"hostname":{"alias":"aaa-opsmatic","id":"hostname"}}`)
	err = ioutil.WriteFile(srcFile, diff1, 0644)
	c.Assert(err, IsNil)

	updated, err := ds.updatePluginInventoryCache(s.plugin, id)
	c.Assert(updated, Equals, true)
	c.Assert(err, IsNil)

	diff2 := []byte(`{"hostname":{"alias":"bbb-opsmatic","id":"hostname"}}`)
	err = ioutil.WriteFile(srcFile, diff2, 0644)
	c.Assert(err, IsNil)

	updated, err = ds.updatePluginInventoryCache(s.plugin, id)
	c.Assert(updated, Equals, true)
	c.Assert(err, IsNil)

	diff3 := []byte(`{"hostname":{"alias":"ccc-opsmatic","id":"hostname"}}`)
	err = ioutil.WriteFile(srcFile, diff3, 0644)
	c.Assert(err, IsNil)

	updated, err = ds.updatePluginInventoryCache(s.plugin, id)
	c.Assert(updated, Equals, true)
	c.Assert(err, IsNil)

	err = ds.SaveState()

	c.Assert(err, IsNil)
	c.Assert(ds.NextIDMap["metadata/plugin"].LastSentID, Equals, int64(0))
	c.Assert(ds.NextIDMap["metadata/plugin"].MostRecentID, Equals, int64(3))

	return ds
}

func (s *DeltaUtilsCoreSuite) TestCompactStoreNoChange(c *C) {
	ds := s.SetupSavedState(c)
	size, err := ds.StorageSize(ds.CacheDir)
	c.Check(err, IsNil)

	err = ds.CompactStorage("", size+1024)

	c.Assert(err, IsNil)
	newSize, err := ds.StorageSize(ds.CacheDir)
	c.Check(err, IsNil)
	c.Assert(size, Equals, newSize)
}

func (s *DeltaUtilsCoreSuite) TestCompactStoreTrimSentDelta(c *C) {
	const id = "entity:ID"

	ds := s.SetupSavedState(c)
	ds.NextIDMap["metadata/plugin"].LastSentID = int64(2)
	err := ds.archivePlugin(ds.NextIDMap["metadata/plugin"], id)
	c.Check(err, IsNil)
	size, err := ds.StorageSize(ds.CacheDir)
	c.Check(err, IsNil)

	err = ds.CompactStorage(id, size-128)

	c.Assert(err, IsNil)
	newSize, err := ds.StorageSize(ds.CacheDir)
	c.Check(err, IsNil)
	c.Assert(newSize < size, Equals, true)
}

func (s *DeltaUtilsCoreSuite) TestCompactStoreRemoveUnusedPlugin(c *C) {
	const id = "entity:ID"

	ds := s.SetupSavedState(c)
	plugin2 := &PluginInfo{
		Source:       "fancy/plugin",
		Plugin:       "fancy",
		FileName:     "plugin.json",
		MostRecentID: int64(0),
		LastSentID:   int64(0),
	}
	fancyFile := ds.SourceFilePath(plugin2, id)
	err := os.MkdirAll(filepath.Dir(fancyFile), 0755)
	c.Check(err, IsNil)
	diffFancy := []byte(`{"fancy":{"alias":"thing1","id":"one"}}`)
	err = ioutil.WriteFile(fancyFile, diffFancy, 0644)
	c.Check(err, IsNil)
	updated, err := ds.updatePluginInventoryCache(plugin2, id)
	c.Assert(updated, Equals, true)
	c.Assert(err, IsNil)
	_, err = os.Stat(ds.SourceFilePath(plugin2, id))
	c.Assert(err, IsNil)
	_, err = os.Stat(ds.cachedFilePath(plugin2, id))
	c.Assert(err, IsNil)
	_, err = os.Stat(ds.DeltaFilePath(plugin2, id))
	c.Assert(err, IsNil)
	err = os.Remove(ds.SourceFilePath(plugin2, id))
	c.Assert(err, IsNil)
	size, err := ds.StorageSize(ds.CacheDir)
	c.Check(err, IsNil)

	err = ds.CompactStorage(id, size-128)
	c.Assert(err, IsNil)

	newSize, err := ds.StorageSize(ds.CacheDir)
	c.Check(err, IsNil)
	c.Assert(newSize < size, Equals, true)
	_, err = os.Stat(ds.SourceFilePath(plugin2, id))
	c.Assert(err, NotNil)
	_, err = os.Stat(ds.cachedFilePath(plugin2, id))
	c.Assert(err, NotNil)
	_, err = os.Stat(ds.DeltaFilePath(plugin2, id))
	c.Assert(err, NotNil)
}

func (s *DeltaUtilsCoreSuite) TestDeltaFileCorrupt(c *C) {
	const id = "entity:ID"
	ds := NewStore(s.repoDir, "default", maxInventorySize)
	srcFile := ds.SourceFilePath(s.plugin, id)
	err := os.MkdirAll(filepath.Dir(srcFile), 0755)
	c.Assert(err, IsNil)
	diff1 := []byte(`{"hostname":{"alias":"aaa-opsmatic","id":"hostname"}}`)
	err = ioutil.WriteFile(srcFile, diff1, 0644)
	c.Assert(err, IsNil)

	updated, err := ds.updatePluginInventoryCache(s.plugin, id)
	c.Assert(err, IsNil)
	c.Assert(updated, Equals, true)

	corruptDeltaFile := ds.DeltaFilePath(s.plugin, id)
	err = os.MkdirAll(filepath.Dir(corruptDeltaFile), 0755)
	c.Assert(err, IsNil)
	corruptDelta := []byte(`{"source":"test/thing","id":1,`)
	err = ioutil.WriteFile(corruptDeltaFile, corruptDelta, 0644)
	c.Check(err, IsNil)

	secondPlugin := &PluginInfo{
		Source:       "metadata/plugin2",
		Plugin:       "metadata2",
		FileName:     "plugin.json",
		MostRecentID: int64(0),
		LastSentID:   int64(0),
	}
	srcFile2 := ds.SourceFilePath(secondPlugin, id)
	err = os.MkdirAll(filepath.Dir(srcFile2), 0755)
	c.Assert(err, IsNil)
	diff2 := []byte(`{"hostname":{"alias":"bbb-opsmatic","id":"ipAddress"}}`)
	err = ioutil.WriteFile(srcFile2, diff2, 0644)
	c.Assert(err, IsNil)

	updated, err = ds.updatePluginInventoryCache(secondPlugin, id)
	c.Assert(updated, Equals, true)
	c.Assert(err, IsNil)

	normalDeltaFile := ds.DeltaFilePath(secondPlugin, id)
	err = os.MkdirAll(filepath.Dir(normalDeltaFile), 0755)
	c.Assert(err, IsNil)
	normalDelta := []byte(`{"source":"test/thing2","id":1,"timestamp":1487182523,"diff":null,"full_diff":false}`)

	err = ioutil.WriteFile(normalDeltaFile, normalDelta, 0644)
	c.Check(err, IsNil)

	_, err = ds.ReadDeltas(id)
	c.Assert(err, NotNil)

	_, err = ds.ReadDeltas(id)
	c.Assert(err, IsNil)

	data, err := ioutil.ReadFile(corruptDeltaFile)
	c.Assert(err, IsNil)
	c.Assert(string(data), Equals, "")

	data, err = ioutil.ReadFile(normalDeltaFile)
	c.Assert(err, IsNil)
	c.Assert(string(data), Equals, `{"source":"test/thing2","id":1,"timestamp":1487182523,"diff":null,"full_diff":false}`)
}

func (s *DeltaUtilsCoreSuite) TestRemoveEntity(c *C) {
	const aPlugin = "aPlugin"
	const anotherPlugin = "anotherPlugin"
	const entityToKeep = "entityToKeep"
	const entityToRemove = "entityToRemove"

	// Given a Store object under a base data directory
	baseDir, err := ioutil.TempDir("", "test-remove-entity")
	c.Assert(err, IsNil)

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
		c.Assert(os.MkdirAll(dir.path, 0755), IsNil)
	}
	store := NewStore(baseDir, "default", maxInventorySize)

	// When removing data from a given entity:
	_ = store.RemoveEntityFolders(entityToRemove)

	// Then only the folders from such entity are removed
	for _, dir := range directories {
		_, err := os.Stat(dir.path)
		if dir.shouldBeRemoved {
			c.Assert(os.IsNotExist(err), Equals, true)
		} else {
			c.Assert(err, IsNil)
		}
	}

	_ = os.RemoveAll(baseDir)
}

func (s *DeltaUtilsCoreSuite) TestScanEntityFolders(c *C) {
	const aPlugin = "aPlugin"
	const anotherPlugin = "anotherPlugin"
	const anEntity = "anEntity"
	const anotherEntity = "anotherEntity"

	// Given a Store object under a base data directory
	baseDir, err := ioutil.TempDir("", "test-scan-entity-folders")
	c.Assert(err, IsNil)

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
		c.Assert(os.MkdirAll(dir.path, 0755), IsNil)
	}
	store := NewStore(baseDir, "default", maxInventorySize)

	// When fetching all the entities
	entities, err := store.ScanEntityFolders()
	c.Assert(err, IsNil)

	// Then only the entities in the system are retrieved, ignoring those from cache and sample dirs
	c.Assert(entities, DeepEquals, map[string]interface{}{"anEntity": true, "anotherEntity": true})

	_ = os.RemoveAll(baseDir)
}

func (s *DeltaUtilsCoreSuite) TestCollectPluginFiles(c *C) {
	const aPlugin = "aPlugin"
	const anotherPlugin = "anotherPlugin"
	const anEntity = "anEntity"
	const anotherEntity = "anotherEntity"

	// Given a Store object under a base data directory
	baseDir, err := ioutil.TempDir("", "test-scan-entity-folders")
	c.Assert(err, IsNil)

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
		c.Assert(os.MkdirAll(filepath.Dir(dir.path), 0755), IsNil)
		file, err := os.Create(dir.path)
		c.Assert(err, IsNil)
		file.Close()
	}
	store := NewStore(baseDir, "default", maxInventorySize)

	// When collecting all the plugins of a given entity
	plugins, err := store.collectPluginFiles(store.DataDir, anEntity, helpers.JsonFilesRegexp)
	c.Assert(err, IsNil)
	// They got the expected values
	c.Assert(len(plugins), Equals, 2)
	c.Assert(plugins[0].Plugin, Not(Equals), plugins[1].Plugin)
	expectedPlugins := map[string]bool{"aPlugin/aFile": true, "anotherPlugin/anotherFile": true}
	c.Assert(expectedPlugins[plugins[0].Source], Equals, true)
	c.Assert(expectedPlugins[plugins[1].Source], Equals, true)

	// When collecting other plugins for the given entity
	plugins, err = store.collectPluginFiles(store.DataDir, anotherEntity, helpers.JsonFilesRegexp)
	c.Assert(err, IsNil)
	c.Assert(len(plugins) > 0, Equals, true)
	// They got the correct ones
	for _, plugin := range plugins {
		c.Assert(plugin.Source, Equals, "aPlugin/aFile")
	}
	os.RemoveAll(baseDir)
}

func (s *DeltaUtilsCoreSuite) TestUpdatePluginInventoryCacheDeltaFileCorrupted(c *C) {
	testCases := []map[string][]byte{{
		"source": []byte(`{"hostname":{"alias":"aaa-opsmatic","id":"hostname"`),
		"cache":  []byte(`{"hostname":{"alias":"aaa-opsmatic","id":"hostname"}}`),
	}, {
		"source": []byte(`{"hostname":{"alias":"aaa-opsmatic","id":"hostname"}}`),
		"cache":  []byte(`{"hostname":{"alias":"aaa-opsmatic","id":"hostname"`),
	}}
	dataDir, err := TempDeltaStoreDir()
	c.Assert(err, IsNil)

	for _, testCase := range testCases {
		// Given corrupted data and deltas
		sourceDir := filepath.Join(dataDir, "corrupted", localEntityFolder)
		sourceJSON := filepath.Join(sourceDir, "corrupted.json")
		cacheDir := filepath.Join(dataDir, CACHE_DIR, "corrupted", localEntityFolder)
		cacheJSON := filepath.Join(cacheDir, "corrupted.json")

		// And a delta storage
		ds := NewStore(dataDir, "default", maxInventorySize)
		c.Assert(os.MkdirAll(sourceDir, 0755), IsNil)
		c.Assert(os.MkdirAll(cacheDir, 0755), IsNil)
		c.Assert(ioutil.WriteFile(sourceJSON, testCase["source"], 0644), IsNil)
		c.Assert(ioutil.WriteFile(cacheJSON, testCase["cache"], 0644), IsNil)

		// When the updatePluginInventoryCache method tries to deal with the corrupted JSONs
		updated, err := ds.updatePluginInventoryCache(&PluginInfo{
			Source:   "corrupted/corrupted",
			Plugin:   "corrupted",
			FileName: "corrupted.json",
		}, "")

		c.Assert(updated, Equals, true)
		c.Assert(err, NotNil)

		// The corrupted plugin is removed
		_, err = os.Stat(cacheJSON)
		c.Assert(os.IsNotExist(err), Equals, true)
		_, err = os.Stat(sourceJSON)
		c.Assert(os.IsNotExist(err), Equals, true)
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
	ds := NewStore(dataPath, "default", maxInventorySize)
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

