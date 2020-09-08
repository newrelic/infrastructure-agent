// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package agent

import (
	ctx "context"
	"fmt"
	"github.com/stretchr/testify/mock"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent/delta"
	"github.com/newrelic/infrastructure-agent/internal/agent/id"
	"github.com/newrelic/infrastructure-agent/internal/testhelpers"
	"github.com/newrelic/infrastructure-agent/pkg/backend/http"
	"github.com/newrelic/infrastructure-agent/pkg/backend/inventoryapi"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	entityKey = "entityKey"
	agentKey  = "agentKey"
)

var (
	agentIdn = entity.Identity{ID: 13}
	endOf18  = time.Date(2018, 12, 12, 12, 12, 12, 12, &time.Location{})
)

func TempDeltaStoreDir() (string, error) {
	return ioutil.TempDir("", "deltastore")
}

func FailingPostDelta(_ []string, _ bool, _ ...*inventoryapi.RawDelta) (*inventoryapi.PostDeltaResponse, error) {
	return nil, fmt.Errorf("trigering an error for post delta API")
}

func FakePostDelta(_ []string, _ bool, _ ...*inventoryapi.RawDelta) (*inventoryapi.PostDeltaResponse, error) {
	return &inventoryapi.PostDeltaResponse{}, nil
}

func ResetPostDelta(_ []string, _ bool, _ ...*inventoryapi.RawDelta) (*inventoryapi.PostDeltaResponse, error) {
	return &inventoryapi.PostDeltaResponse{
		Reset: inventoryapi.ResetAll,
	}, nil
}

func TestNewPatchSender(t *testing.T) {
	assert.Implements(t, (*patchSender)(nil), newTestPatchSender(t, "", &delta.Store{}, delta.NewLastSubmissionInMemory(), nil))
}

func cachePluginData(t *testing.T, store *delta.Store, entityKey string) {
	plugin := &delta.PluginInfo{
		Source:   "metadata/plugin",
		Plugin:   "metadata",
		FileName: "plugin.json",
	}
	srcFile := store.SourceFilePath(plugin, entityKey)
	err := os.MkdirAll(filepath.Dir(srcFile), 0755)
	assert.NoError(t, err)
	diff1 := []byte(`{"hostname":{"alias":"aaa-opsmatic","entityKey":"hostname"}}`)
	err = ioutil.WriteFile(srcFile, diff1, 0644)
	assert.NoError(t, err)
	err = store.UpdatePluginsInventoryCache(entityKey)
	assert.NoError(t, err)
	err = store.SaveState()
	assert.NoError(t, err)
	fileInfo, err := os.Stat(filepath.Join(store.CacheDir, "metadata", entityKey))
	assert.NoError(t, err)
	assert.True(t, fileInfo.IsDir())
}

func TestPatchSender_Process_LongTermOffline(t *testing.T) {
	// Given a delta Store
	dataDir, err := TempDeltaStoreDir()
	assert.NoError(t, err)
	store := delta.NewStore(dataDir, "default", maxInventoryDataSize)
	ls := delta.NewLastSubmissionInMemory()

	// With some cached plugin data
	cachePluginData(t, store, entityKey)

	// And a patch sender that has been disconnected for more than 24 hours
	ps := newTestPatchSender(t, dataDir, store, ls, nil)
	nowIsEndOf18()
	duration25h, _ := time.ParseDuration("25h")
	assert.NoError(t, ls.UpdateTime(timeNow().Add(-duration25h)))

	// When the patch sender tries to process the deltas
	err = ps.Process()

	// It returns an error since they are not sent, but just cleaned up
	assert.Error(t, err)

	// And the delta cache has been cleaned up
	_, err = os.Stat(filepath.Join(store.CacheDir, "metadata", entityKey))
	assert.True(t, os.IsNotExist(err), "err: %+v", err)
}

func TestPatchSender_Process_LongTermOffline_ReconnectPlugins(t *testing.T) {
	// Given a delta Store
	dataDir, err := TempDeltaStoreDir()
	assert.NoError(t, err)
	store := delta.NewStore(dataDir, "default", maxInventoryDataSize)
	ls := delta.NewLastSubmissionInMemory()

	// With some cached plugin data
	cachePluginData(t, store, entityKey)

	// And a patch sender that has been disconnected for more than 24 hours, but doesn't need to reset deltas
	ps := newTestPatchSender(t, dataDir, store, ls, nil)
	ps.postDeltas = FakePostDelta
	ps.lastDeltaRemoval = endOf18
	var agentKey atomic.Value
	agentKey.Store("test")
	ps.context = &context{
		reconnecting: new(sync.Map),
		agentKey:     agentKey,
	}

	nowIsEndOf18()
	duration25h, _ := time.ParseDuration("25h")
	assert.NoError(t, ls.UpdateTime(timeNow().Add(-duration25h)))

	// With a re-connectable plugin
	wg := &sync.WaitGroup{}
	plugin := reconnectingPlugin{context: ps.context, invocations: 0, wg: wg}
	ps.context.AddReconnecting(&plugin)
	wg.Add(1)

	// When the patch sender tries to process the deltas
	assert.NoError(t, ps.Process())

	// The registered plugin has been invoked to run again
	assert.NoError(t, wait(3*time.Second, wg))
	assert.Equal(t, 1, plugin.invocations)
}

func TestPatchSender_Process_LongTermOffline_NoDeltasToPost_UpdateLastDeltaRemoval(t *testing.T) {
	// Given a delta Store
	dataDir, err := TempDeltaStoreDir()
	assert.NoError(t, err)
	store := delta.NewStore(dataDir, "default", maxInventoryDataSize)
	ls := delta.NewLastSubmissionInMemory()
	// When it has successfully submitted some deltas
	require.NoError(t, ls.UpdateTime(time.Now()))

	ps := newTestPatchSender(t, dataDir, store, ls, nil)
	ps.postDeltas = FailingPostDelta
	ps.lastDeltaRemoval = time.Date(2018, 12, 12, 0, 12, 12, 12, &time.Location{})
	// And a patch sender that has been disconnected for less than 24 hours
	resetTime, _ := time.ParseDuration("24h")
	ps.resetIfOffline = resetTime

	nowIsEndOf18()
	assert.NoError(t, ls.UpdateTime(timeNow()))

	// When the patch sender tries to process the deltas
	err = ps.Process()

	// The lastDeltaRemoval time has been updated
	var lastConn time.Time
	lastConn, err = ls.Time()
	require.NoError(t, err)
	assert.True(t, ps.lastDeltaRemoval.Before(lastConn))
}

func TestPatchSender_Process_LongTermOffline_AlreadyRemoved(t *testing.T) {
	// Given a delta Store
	dataDir, err := TempDeltaStoreDir()
	assert.NoError(t, err)
	store := delta.NewStore(dataDir, "default", maxInventoryDataSize)
	ls := delta.NewLastSubmissionInMemory()

	// With some cached plugin data
	cachePluginData(t, store, entityKey)

	// And a patch sender that has been disconnected for more than 24 hours
	resetTime, _ := time.ParseDuration("24h")
	// But the deltas were already cleaned up less than 24 hours ago
	lastRemoval := time.Date(2018, 12, 12, 10, 12, 12, 12, &time.Location{})
	ps := newTestPatchSender(t, dataDir, store, ls, nil)
	ps.postDeltas = FailingPostDelta
	ps.lastDeltaRemoval = lastRemoval
	ps.resetIfOffline = resetTime

	nowIsEndOf18()
	assert.NoError(t, ls.UpdateTime(timeNow()))

	// When the patch sender tries to process the deltas
	err = ps.Process()

	// It returns an error since they are not sent
	assert.Error(t, err)

	// But the current delta cache is not cleaned up since it is less than 24 hours old
	fileInfo, err := os.Stat(filepath.Join(store.CacheDir, "metadata", entityKey))
	assert.NoError(t, err)
	assert.True(t, fileInfo.IsDir())
}

type mockLastSubmissionTime struct {
	mock.Mock
}

func (l *mockLastSubmissionTime) Time() (time.Time, error) {
	return l.Called().Get(0).(time.Time), l.Called().Get(1).(error)
}

func (l *mockLastSubmissionTime) UpdateTime(time time.Time) error {
	l.Called(time)
	return nil
}

func TestPatchSender_Process_LastSubmissionTime_IgnoreEmptyEntityKey(t *testing.T) {
	rawDelta := []inventoryapi.RawDeltaBlock{{&inventoryapi.RawDelta{ID: 1234}}}

	// GIVEN a delta store
	storage := &mockStorage{}
	storage.On("ReadDeltas", mock.Anything).Return(rawDelta)

	// AND a submission time storage handler
	lastSubmissionTime := &mockLastSubmissionTime{}
	lastSubmissionTime.On("UpdateTime", mock.Anything).Return()

	// AND a patchSender
	sender := newTestPatchSender(t, "", storage, lastSubmissionTime, &mockEntityIDPersist{})
	sender.entityKey = ""
	sender.compactEnabled = false
	sender.postDeltas = assertDeltaSent(t, rawDelta)

	// WHEN process deltas
	err := sender.Process()
	require.NoError(t, err)

	// THEN last submission time was not updated
	lastSubmissionTime.AssertNotCalled(t, "UpdateTime", mock.Anything)
}

func assertDeltaSent(t *testing.T, rawDelta []inventoryapi.RawDeltaBlock) func(entityKeys []string, isAgent bool, deltas ...*inventoryapi.RawDelta) (*inventoryapi.PostDeltaResponse, error) {
	return func(entityKeys []string, isAgent bool, deltas ...*inventoryapi.RawDelta) (*inventoryapi.PostDeltaResponse, error) {
		// AND deltas are sent to the backend
		for _, raw := range rawDelta {
			assert.ElementsMatch(t, raw, deltas)
		}
		return &inventoryapi.PostDeltaResponse{Reset: "no"}, nil
	}
}

func TestPatchSender_Process_ShortTermOffline(t *testing.T) {
	// Given a delta Store
	dataDir, err := TempDeltaStoreDir()
	assert.NoError(t, err)
	store := delta.NewStore(dataDir, "default", maxInventoryDataSize)
	ls := delta.NewLastSubmissionInMemory()

	// With some cached plugin data
	cachePluginData(t, store, entityKey)

	// And a patch sender that has been disconnected for less than 24 hours
	resetTime, _ := time.ParseDuration("24h")
	lastDeltaRemoval := time.Date(2018, 12, 12, 0, 12, 12, 12, &time.Location{})
	ps := newTestPatchSender(t, dataDir, store, ls, nil)
	ps.postDeltas = FailingPostDelta
	ps.lastDeltaRemoval = lastDeltaRemoval
	ps.resetIfOffline = resetTime

	nowIsEndOf18()
	assert.NoError(t, ls.UpdateTime(timeNow()))

	// When the patch sender fails at processing deltas
	err = ps.Process()
	assert.Error(t, err)

	// The delta cache has NOT been cleaned up
	fileInfo, err := os.Stat(filepath.Join(store.CacheDir, "metadata", entityKey))
	assert.NoError(t, err)
	assert.True(t, fileInfo.IsDir())
}

func TestPatchSender_Process_DividedDeltas(t *testing.T) {
	// Given a patch sender
	dataDir, err := TempDeltaStoreDir()
	assert.NoError(t, err)

	nowIsEndOf18()

	store := delta.NewStore(dataDir, "localhost", maxInventoryDataSize)
	ls := delta.NewLastSubmissionInMemory()
	require.NoError(t, ls.UpdateTime(timeNow()))
	ps := newTestPatchSender(t, dataDir, store, ls, nil)
	pdt := testhelpers.NewPostDeltaTracer(maxInventoryDataSize)
	ps.postDeltas = pdt.PostDeltas
	ps.lastDeltaRemoval = time.Date(2018, 12, 12, 0, 12, 12, 12, &time.Location{})
	resetTime, _ := time.ParseDuration("24h")
	ps.resetIfOffline = resetTime

	// And a set of normal-sized deltas from different plugins
	testhelpers.PopulateDeltas(dataDir, entityKey, []testhelpers.FakeDeltaEntry{
		{Source: "plugin1/plugin1", DeltasSize: maxInventoryDataSize/3 + 100, BodySize: 10000},
		{Source: "plugin2/plugin2", DeltasSize: maxInventoryDataSize/3 + 100, BodySize: 1000},
		{Source: "plugin3/plugin3", DeltasSize: maxInventoryDataSize/3 + 100, BodySize: 1000},
	})

	// When the patch sender processes them
	assert.NoError(t, ps.Process())

	// They are divided in chunks and submitted in multiple invocations
	assert.Len(t, pdt.Errors, 0)
	assert.Len(t, pdt.Sources, 2)

	// Two plugins deltas in the first invocation
	assert.Contains(t, pdt.Sources[0], "plugin1/plugin1")
	assert.Contains(t, pdt.Sources[0], "plugin2/plugin2")

	// The later plugin in the second invocation
	assert.Contains(t, pdt.Sources[1], "plugin3/plugin3")
}

func TestPatchSender_Process_DisabledDeltaSplit(t *testing.T) {
	// Given a patch sender with disabled delta split
	dataDir, err := TempDeltaStoreDir()
	assert.NoError(t, err)
	store := delta.NewStore(dataDir, "localhost", delta.DisableInventorySplit)
	ps := newTestPatchSender(t, dataDir, store, delta.NewLastSubmissionInMemory(), nil)
	pdt := testhelpers.NewPostDeltaTracer(math.MaxInt32)
	ps.postDeltas = pdt.PostDeltas

	// And a set of normal-sized deltas from different plugins
	testhelpers.PopulateDeltas(dataDir, entityKey, []testhelpers.FakeDeltaEntry{
		{Source: "plugin1/plugin1", DeltasSize: maxInventoryDataSize/3 + 100, BodySize: 10000},
		{Source: "plugin2/plugin2", DeltasSize: maxInventoryDataSize/3 + 100, BodySize: 1000},
		{Source: "plugin3/plugin3", DeltasSize: maxInventoryDataSize/3 + 100, BodySize: 1000},
	})

	// When the patch sender processes them
	assert.NoError(t, ps.Process())

	// They are not divided in chunks even if they are larger than the maximum inventory data size
	assert.Len(t, pdt.Errors, 0)
	assert.Len(t, pdt.Sources, 1)

	// And all the deltas are sent in the same invocation
	assert.Contains(t, pdt.Sources[0], "plugin1/plugin1")
	assert.Contains(t, pdt.Sources[0], "plugin2/plugin2")
	assert.Contains(t, pdt.Sources[0], "plugin3/plugin3")
}

func TestPatchSender_Process_SingleRequestDeltas(t *testing.T) {
	// Given a patch sender
	dataDir, err := TempDeltaStoreDir()
	assert.NoError(t, err)
	store := delta.NewStore(dataDir, "localhost", maxInventoryDataSize)
	ls := delta.NewLastSubmissionInMemory()

	pdt := testhelpers.NewPostDeltaTracer(maxInventoryDataSize)
	resetTime, _ := time.ParseDuration("24h")
	lastDeltaRemoval := time.Date(2018, 12, 12, 0, 12, 12, 12, &time.Location{})
	ps := newTestPatchSender(t, dataDir, store, ls, nil)
	ps.postDeltas = pdt.PostDeltas
	ps.lastDeltaRemoval = lastDeltaRemoval
	ps.resetIfOffline = resetTime

	// And a set of deltas from different plugins, whose total size is smaller than the max inventory data size
	testhelpers.PopulateDeltas(dataDir, entityKey, []testhelpers.FakeDeltaEntry{
		{Source: "plugin1/plugin1", DeltasSize: maxInventoryDataSize / 10, BodySize: 100},
		{Source: "plugin2/plugin2", DeltasSize: maxInventoryDataSize / 10, BodySize: 100},
		{Source: "plugin3/plugin3", DeltasSize: maxInventoryDataSize / 10, BodySize: 100},
		{Source: "plugin4/plugin4", DeltasSize: maxInventoryDataSize / 10, BodySize: 100},
	})

	// When the patch sender processes them
	assert.NoError(t, ps.Process())

	// They are sent in a single request
	assert.Len(t, pdt.Errors, 0)
	assert.Len(t, pdt.Sources, 1)

	// All in the first invocation
	assert.Contains(t, pdt.Sources[0], "plugin1/plugin1")
	assert.Contains(t, pdt.Sources[0], "plugin2/plugin2")
	assert.Contains(t, pdt.Sources[0], "plugin3/plugin3")
	assert.Contains(t, pdt.Sources[0], "plugin4/plugin4")
}

func TestPatchSender_Process_CompactEnabled(t *testing.T) {
	// Given a patch sender with compaction enabled
	dataDir, err := TempDeltaStoreDir()
	assert.NoError(t, err)
	store := delta.NewStore(dataDir, "localhost", maxInventoryDataSize)
	ls := delta.NewLastSubmissionInMemory()

	resetTime, _ := time.ParseDuration("24h")
	lastDeltaRemoval := time.Date(2018, 12, 12, 0, 12, 12, 12, &time.Location{})
	ps := newTestPatchSender(t, dataDir, store, ls, nil)
	ps.postDeltas = FakePostDelta
	ps.lastDeltaRemoval = lastDeltaRemoval
	ps.resetIfOffline = resetTime
	ps.compactEnabled = true

	// And a set of stored deltas that occupy a given size in disk
	testhelpers.PopulateDeltas(dataDir, entityKey, []testhelpers.FakeDeltaEntry{
		{Source: "plugin1/plugin1", DeltasSize: maxInventoryDataSize / 10, BodySize: 100},
		{Source: "plugin2/plugin2", DeltasSize: maxInventoryDataSize / 10, BodySize: 100},
		{Source: "plugin3/plugin3", DeltasSize: maxInventoryDataSize / 10, BodySize: 100},
		{Source: "plugin4/plugin4", DeltasSize: maxInventoryDataSize / 10, BodySize: 100},
	})
	storageSize, err := store.StorageSize(dataDir)
	assert.NoError(t, err)

	// When the patch sender processes them
	assert.NoError(t, ps.Process())

	// The data is compacted
	compactedSize, err := store.StorageSize(dataDir)
	assert.NoError(t, err)
	assert.True(t, compactedSize < storageSize, "%v not smaller than %v", compactedSize, storageSize)
}

func TestPatchSender_Process_Reset(t *testing.T) {
	// Given a patch sender
	dataDir, err := TempDeltaStoreDir()
	assert.NoError(t, err)
	store := delta.NewStore(dataDir, "localhost", maxInventoryDataSize)
	ls := delta.NewLastSubmissionInMemory()

	resetTime, _ := time.ParseDuration("24h")
	lastDeltaRemoval := time.Date(2018, 12, 12, 0, 12, 12, 12, &time.Location{})
	ps := newTestPatchSender(t, dataDir, store, ls, nil)
	// And a backend service that returns ResetAll after being invoked
	ps.postDeltas = ResetPostDelta
	ps.lastDeltaRemoval = lastDeltaRemoval
	ps.resetIfOffline = resetTime
	var agentKeyVal atomic.Value
	agentKeyVal.Store(agentKey)
	ps.context = &context{agentKey: agentKeyVal, reconnecting: new(sync.Map)}
	ps.compactEnabled = true

	// And a set of stored deltas that occupy a given size in disk
	testhelpers.PopulateDeltas(dataDir, entityKey, []testhelpers.FakeDeltaEntry{
		{Source: "plugin1/plugin1", DeltasSize: maxInventoryDataSize / 10, BodySize: 100},
		{Source: "plugin2/plugin2", DeltasSize: maxInventoryDataSize / 10, BodySize: 100},
		{Source: "plugin3/plugin3", DeltasSize: maxInventoryDataSize / 10, BodySize: 100},
		{Source: "plugin4/plugin4", DeltasSize: maxInventoryDataSize / 10, BodySize: 100},
	})

	// When the patch sender processes them
	assert.NoError(t, ps.Process())

	// The deltas are removed
	storageSize, err := store.StorageSize(dataDir)
	assert.NoError(t, err)

	// few bytes remain (the almost-empty .delta_repo/delta_id_file.json file) + few directories
	// add length of a last_submission file
	expectedStorageSize := len("{}")
	expectedStorageSize += len("2020-07-02T18:12:02.921062+02:00")
	assert.True(t, storageSize <= uint64(expectedStorageSize), "%v not smaller or equal than %d", storageSize, expectedStorageSize)
}

func TestPathSender_Process_EntityIDChanged_ResetLocalEntityDeltas(t *testing.T) {
	newIdentity := entity.Identity{
		ID: entity.ID(23456),
	}

	// GIVEN a delta store
	storage := &mockStorage{}
	storage.On("ReadDeltas", mock.Anything).Return([]inventoryapi.RawDeltaBlock{})
	storage.On("RemoveEntity", agentKey).Return(nil)

	// AND a disk persisted entityID
	lastEntityID := &mockEntityIDPersist{}
	lastEntityID.On("UpdateEntityID", newIdentity.ID).Return(nil)
	lastEntityID.On("GetEntityID").Return(entity.ID(654321))

	// AND a patchSender
	sender := newTestPatchSender(t, "", storage, delta.NewLastSubmissionInMemory(), lastEntityID)
	sender.entityKey = agentKey

	// AND set a new identity
	idCtx := id.NewContext(ctx.Background())
	sender.agentIDProvide = idCtx.AgentIdentity
	idCtx.SetAgentIdentity(newIdentity)

	// WHEN process deltas
	sender.Process()

	// THEN remove deltas for the entity and update the persisted entityID
	storage.AssertCalled(t, "RemoveEntity", agentKey)
	lastEntityID.AssertCalled(t, "UpdateEntityID", newIdentity.ID)
}

func TestPathSender_Process_EmptyEntityID_UpdateEntityIDWithCurrentAgentID(t *testing.T) {
	agentID := entity.Identity{
		ID: entity.ID(23456),
	}

	// GIVEN a delta store
	storage := &mockStorage{}
	storage.On("ReadDeltas", mock.Anything).Return([]inventoryapi.RawDeltaBlock{})
	storage.On("RemoveEntity", agentKey).Return(nil)

	// AND no persisted entityID
	lastEntityID := &mockEntityIDPersist{}
	lastEntityID.On("UpdateEntityID", agentID.ID).Return(nil)
	lastEntityID.On("GetEntityID").Return(entity.EmptyID)

	// AND a patchSender
	sender := newTestPatchSender(t, "", storage, delta.NewLastSubmissionInMemory(), lastEntityID)
	sender.entityKey = agentKey

	// AND set a new identity
	idCtx := id.NewContext(ctx.Background())
	sender.agentIDProvide = idCtx.AgentIdentity
	idCtx.SetAgentIdentity(agentID)

	// WHEN process deltas
	sender.Process()

	// THEN we should not remove deltas and update the persisted entityID
	storage.AssertNotCalled(t, "RemoveEntity", agentKey)
	lastEntityID.AssertCalled(t, "UpdateEntityID", agentID.ID)
}

type mockStorage struct {
	mock.Mock
	delta.Storage
}

func (m *mockStorage) RemoveEntity(entityKey string) error {
	m.Called(entityKey)
	return nil
}

func (m *mockStorage) ReadDeltas(entityKey string) ([]inventoryapi.RawDeltaBlock, error) {
	return m.Called(entityKey).Get(0).([]inventoryapi.RawDeltaBlock), nil
}

func (m *mockStorage) UpdateState(entityKey string, deltas []*inventoryapi.RawDelta, deltaStateResults *inventoryapi.DeltaStateMap) {
}

type mockEntityIDPersist struct {
	mock.Mock
}

func (e *mockEntityIDPersist) GetEntityID() (entity.ID, error) {
	return e.Called().Get(0).(entity.ID), nil
}

func (e *mockEntityIDPersist) UpdateEntityID(id entity.ID) error {
	e.Called(id)
	return nil
}

func newTestPatchSender(t *testing.T, dataDir string, store delta.Storage, ls delta.LastSubmissionStore, lastEntityID delta.EntityIDPersist) *patchSenderIngest {
	idCtx := id.NewContext(ctx.Background())
	var agentKeyVal atomic.Value
	agentKeyVal.Store(agentKey)
	aCtx := &context{
		agentKey: agentKeyVal,
		cfg:      config.NewTestWithDeltas(dataDir),
	}
	psI, err := newPatchSender(
		entityKey,
		aCtx,
		store,
		ls,
		lastEntityID,
		"user agent",
		idCtx.AgentIdnOrEmpty,
		http.NullHttpClient,
	)
	require.NoError(t, err)
	ps := psI.(*patchSenderIngest)
	return ps
}

func nowIsEndOf18() {
	timeNow = func() time.Time {
		return endOf18
	}
}
