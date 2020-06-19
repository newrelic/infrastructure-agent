// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package agent

import (
	"fmt"
	"math"
	http2 "net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent/delta"
	"github.com/newrelic/infrastructure-agent/internal/testhelpers"
	"github.com/newrelic/infrastructure-agent/pkg/backend/http"
	"github.com/newrelic/infrastructure-agent/pkg/backend/inventoryapi"
	"github.com/newrelic/infrastructure-agent/pkg/backend/state"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	infra "github.com/newrelic/infrastructure-agent/test/infra/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func FailingPostDeltaVortex(_ entity.ID, _ []string, _ bool, _ ...*inventoryapi.RawDelta) (*inventoryapi.PostDeltaResponse, error) {
	return nil, fmt.Errorf("catapun!")
}

func FakePostDeltaVortex(_ entity.ID, _ []string, _ bool, _ ...*inventoryapi.RawDelta) (*inventoryapi.PostDeltaResponse, error) {
	return &inventoryapi.PostDeltaResponse{}, nil
}

func ResetPostDeltaVortex(_ entity.ID, _ []string, _ bool, _ ...*inventoryapi.RawDelta) (*inventoryapi.PostDeltaResponse, error) {
	return &inventoryapi.PostDeltaResponse{
		Reset: inventoryapi.ResetAll,
	}, nil
}

func TestNewPatchSenderVortex(t *testing.T) {
	ps := newSender(t, newContextWithVortex(), &delta.Store{}, http.NullHttpClient)
	assert.NotNil(t, ps)
}

func TestPatchSenderVortex_Process_LongTermOffline(t *testing.T) {
	// Given a delta Store
	dataDir, err := TempDeltaStoreDir()
	assert.NoError(t, err)
	store := delta.NewStore(dataDir, "default", maxInventoryDataSize)
	// With some cached plugin data
	cachePluginData(t, store, "entityKey")

	// And a patch sender that has been disconnected for more than 24 hours
	psV := newSender(t, newContextWithVortex(), store, http.NullHttpClient)
	ps := psV.(*patchSenderVortex)
	lastConnection := time.Date(2018, 12, 10, 12, 12, 12, 12, &time.Location{})
	ps.lastConnection = lastConnection
	ps.lastDeltaRemoval = lastConnection

	// When the patch sender tries to process the deltas
	// It returns an error since they are not sent, but just cleaned up
	assert.Error(t, ps.Process())

	// And the delta cache has been cleaned up
	_, err = os.Stat(filepath.Join(store.CacheDir, "metadata", "entityKey"))
	assert.True(t, os.IsNotExist(err))
}

func TestPatchSenderVortex_Process_LongTermOffline_ReconnectPlugins(t *testing.T) {
	// Given a delta Store
	dataDir, err := TempDeltaStoreDir()
	assert.NoError(t, err)
	store := delta.NewStore(dataDir, "default", maxInventoryDataSize)

	// With some cached plugin data
	cachePluginData(t, store, "entityKey")

	// And a patch sender that has been disconnected for more than 24 hours, but doesn't need to reset deltas
	resetTime, _ := time.ParseDuration("24h")
	lastConnection := time.Date(2018, 12, 10, 12, 12, 12, 12, &time.Location{})
	lastDeltaRemoval := time.Date(2018, 12, 12, 12, 12, 12, 12, &time.Location{})

	ctx := newContextWithVortex()
	ctx.reconnecting = new(sync.Map)

	psV := newSender(t, ctx, store, http.NullHttpClient)
	ps := psV.(*patchSenderVortex)
	ps.postDeltas = FakePostDeltaVortex
	ps.lastConnection = lastConnection
	ps.lastDeltaRemoval = lastDeltaRemoval
	ps.resetIfOffline = resetTime

	timeNowVortex = func() time.Time {
		return time.Date(2018, 12, 12, 12, 12, 12, 12, &time.Location{})
	}

	// With a reconnectable plugin
	wg := &sync.WaitGroup{}
	plugin := reconnectingPlugin{context: ps.context, invocations: 0, wg: wg}
	ps.context.AddReconnecting(&plugin)
	wg.Add(1)

	// When the patch sender tries to process the deltas
	require.NoError(t, ps.Process())

	// The registered plugin has been invoked to run again
	assert.NoError(t, wait(5*time.Second, wg))
	assert.Equal(t, 1, plugin.invocations)
}

func TestPatchSenderVortex_Process_LongTermOffline_NoDeltasToPost_UpdateLastConnection(t *testing.T) {
	// Given a delta Store
	dataDir, err := TempDeltaStoreDir()
	assert.NoError(t, err)
	store := delta.NewStore(dataDir, "default", maxInventoryDataSize)

	// And a patch sender that has been disconnected for less than 24 hours
	resetTime, _ := time.ParseDuration("24h")
	lastConnection := time.Date(2018, 12, 12, 0, 12, 12, 12, &time.Location{})

	psV := newSender(t, newContextWithVortex(), store, http.NullHttpClient)
	ps := psV.(*patchSenderVortex)
	ps.postDeltas = FailingPostDeltaVortex
	ps.lastConnection = lastConnection
	ps.lastDeltaRemoval = lastConnection
	ps.resetIfOffline = resetTime

	timeNowVortex = func() time.Time {
		return time.Date(2018, 12, 12, 12, 12, 12, 12, &time.Location{})
	}

	// When the patch sender tries to process the deltas
	assert.NoError(t, ps.Process())

	// The lastConnection time has been updated
	assert.True(t, lastConnection.Before(ps.lastConnection))
}

func TestPatchSenderVortex_Process_LongTermOffline_AlreadyRemoved(t *testing.T) {
	// Given a delta Store
	dataDir, err := TempDeltaStoreDir()
	assert.NoError(t, err)
	store := delta.NewStore(dataDir, "default", maxInventoryDataSize)

	// With some cached plugin data
	cachePluginData(t, store, "entityKey")

	// And a patch sender that has been disconnected for more than 24 hours
	psV := newSender(t, newContextWithVortex(), store, http.NullHttpClient)
	ps := psV.(*patchSenderVortex)
	ps.postDeltas = FailingPostDeltaVortex
	ps.lastConnection = time.Date(2018, 12, 10, 12, 12, 12, 12, &time.Location{})
	ps.lastDeltaRemoval = time.Date(2018, 12, 12, 10, 12, 12, 12, &time.Location{})
	resetTime, _ := time.ParseDuration("24h")
	ps.resetIfOffline = resetTime

	timeNowVortex = func() time.Time {
		return time.Date(2018, 12, 12, 12, 12, 12, 12, &time.Location{})
	}

	// When the patch sender tries to process the deltas
	assert.Error(t, ps.Process(), "error should be returned as they are not sent")

	// But the current delta cache is not cleaned up since it is less than 24 hours old
	fileInfo, err := os.Stat(filepath.Join(store.CacheDir, "metadata", "entityKey"))
	require.NoError(t, err)
	assert.True(t, fileInfo.IsDir())
}

func TestPatchSenderVortex_Process_ShortTermOffline(t *testing.T) {
	// Given a delta Store
	dataDir, err := TempDeltaStoreDir()
	assert.NoError(t, err)
	store := delta.NewStore(dataDir, "default", maxInventoryDataSize)
	// With some cached plugin data
	cachePluginData(t, store, "entityKey")

	ctx := newContextWithVortex()
	psV := newSender(t, ctx, store, http.NullHttpClient)

	// And a patch sender that has been disconnected for less than 24 hours
	ps := psV.(*patchSenderVortex)
	lastConnection := time.Date(2018, 12, 12, 0, 12, 12, 12, &time.Location{})
	ps.lastConnection = lastConnection
	ps.lastDeltaRemoval = lastConnection

	resetTime, _ := time.ParseDuration("24h")
	ps.resetIfOffline = resetTime
	ps.postDeltas = FailingPostDeltaVortex

	timeNowVortex = func() time.Time {
		return time.Date(2018, 12, 12, 12, 12, 12, 12, &time.Location{})
	}

	// When the patch sender fails at processing deltas
	assert.Error(t, ps.Process())

	// The delta cache has NOT been cleaned up
	fileInfo, err := os.Stat(filepath.Join(store.CacheDir, "metadata", "entityKey"))
	assert.NoError(t, err)
	assert.True(t, fileInfo.IsDir())
}

func TestPatchSenderVortex_Process(t *testing.T) {
	dataDir, err := TempDeltaStoreDir()
	assert.NoError(t, err)
	store := delta.NewStore(dataDir, "localhost", maxInventoryDataSize)
	// set of deltas from different plugins, whose total size is smaller than the max inventory data size
	testhelpers.PopulateDeltas(dataDir, "entityKey", []testhelpers.FakeDeltaEntry{
		{Source: "plugin1/plugin1", DeltasSize: maxInventoryDataSize / 10, BodySize: 100},
		{Source: "plugin2/plugin2", DeltasSize: maxInventoryDataSize / 10, BodySize: 100},
	})

	ctx := newContextWithVortex()
	pdt := testhelpers.NewPostDeltaTracer(maxInventoryDataSize)

	pSender := newSender(t, ctx, store, http.NullHttpClient)
	ps := pSender.(*patchSenderVortex)
	ps.postDeltas = pdt.PostDeltasVortex

	assert.NoError(t, ps.Process())

	assert.Len(t, pdt.Errors, 0)
	assert.Len(t, pdt.Sources, 1, "just a single request should be used")
	firstRequestSource := pdt.Sources[0]
	assert.Contains(t, firstRequestSource, "plugin1/plugin1")
	assert.Contains(t, firstRequestSource, "plugin2/plugin2")
}

func TestPatchSenderVortex_Process_WaitsForAgentID(t *testing.T) {
	dataDir, err := TempDeltaStoreDir()
	assert.NoError(t, err)
	store := delta.NewStore(dataDir, "localhost", maxInventoryDataSize)
	testhelpers.PopulateDeltas(dataDir, "entityKey", []testhelpers.FakeDeltaEntry{
		{Source: "test/dummy", DeltasSize: maxInventoryDataSize, BodySize: 100},
	})

	rc := infra.NewRequestRecorderClient(infra.AcceptedResponse("test/dummy", 0))

	ctxNoID := newContextWithVortex()
	ctxNoID.SetAgentIdentity(entity.EmptyIdentity) // empty
	ps := newSender(t, ctxNoID, store, rc.Client)

	// Process function is blocked without agent id
	go func() {
		assert.NoError(t, ps.Process())
	}()

	// waiter for incoming request
	ready := make(chan struct{})
	var req http2.Request
	go func() {
		req = <-rc.RequestCh
		close(ready)
	}()

	assert.Empty(t, req, "no inventory request should be sent until there is an agent-id set")

	ctxNoID.SetAgentIdentity(entity.Identity{ID: 123})
	// allow sender to unblock
	<-ready

	assert.NotEmpty(t, req)
}

func TestPatchSenderVortex_Process_DividedDeltas(t *testing.T) {
	// Given a patch sender
	dataDir, err := TempDeltaStoreDir()
	assert.NoError(t, err)
	store := delta.NewStore(dataDir, "localhost", maxInventoryDataSize)
	pdt := testhelpers.NewPostDeltaTracer(maxInventoryDataSize)
	ctx := newContextWithVortex()

	pSender := newSender(t, ctx, store, http.NullHttpClient)
	ps := pSender.(*patchSenderVortex)
	ps.postDeltas = pdt.PostDeltasVortex

	// And a set of normal-sized deltas from different plugins
	testhelpers.PopulateDeltas(dataDir, "entityKey", []testhelpers.FakeDeltaEntry{
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

func TestPatchSenderVortex_Process_DisabledDeltaSplit(t *testing.T) {
	dataDir, err := TempDeltaStoreDir()
	assert.NoError(t, err)
	store := delta.NewStore(dataDir, "localhost", delta.DisableInventorySplit)

	// Given a patch sender with disabled delta split
	pdt := testhelpers.NewPostDeltaTracer(math.MaxInt32)
	ctx := newContextWithVortex()
	pSender := newSender(t, ctx, store, http.NullHttpClient)
	ps := pSender.(*patchSenderVortex)
	ps.postDeltas = pdt.PostDeltasVortex

	// And a set of normal-sized deltas from different plugins
	testhelpers.PopulateDeltas(dataDir, "entityKey", []testhelpers.FakeDeltaEntry{
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

func TestPatchSenderVortex_Process_SingleRequestDeltas(t *testing.T) {
	dataDir, err := TempDeltaStoreDir()
	assert.NoError(t, err)
	store := delta.NewStore(dataDir, "localhost", maxInventoryDataSize)

	// Given a patch sender
	pdt := testhelpers.NewPostDeltaTracer(maxInventoryDataSize)
	ctx := newContextWithVortex()
	pSender := newSender(t, ctx, store, http.NullHttpClient)
	ps := pSender.(*patchSenderVortex)
	ps.postDeltas = pdt.PostDeltasVortex
	// And a set of deltas from different plugins, whose total size is smaller than the max inventory data size
	testhelpers.PopulateDeltas(dataDir, "entityKey", []testhelpers.FakeDeltaEntry{
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

func TestPatchSenderVortex_Process_CompactEnabled(t *testing.T) {
	// Given a patch sender with compaction enabled
	dataDir, err := TempDeltaStoreDir()
	assert.NoError(t, err)
	store := delta.NewStore(dataDir, "localhost", maxInventoryDataSize)

	ctx := newContextWithVortex()
	pSender := newSender(t, ctx, store, http.NullHttpClient)
	ps := pSender.(*patchSenderVortex)
	ps.postDeltas = FakePostDeltaVortex
	ps.compactEnabled = true

	// And a set of stored deltas that occupy a given size in disk
	testhelpers.PopulateDeltas(dataDir, "entityKey", []testhelpers.FakeDeltaEntry{
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

func TestPatchSenderVortex_Process_Reset(t *testing.T) {
	// Given a patch sender
	dataDir, err := TempDeltaStoreDir()
	assert.NoError(t, err)
	store := delta.NewStore(dataDir, "localhost", maxInventoryDataSize)

	resetTime, _ := time.ParseDuration("24h")
	lastConnection := time.Date(2018, 12, 12, 0, 12, 12, 12, &time.Location{})
	ctx := newContextWithVortex()
	ctx.cfg.CompactEnabled = true

	pSender := newSender(t, ctx, store, http.NullHttpClient)
	ps := pSender.(*patchSenderVortex)
	ps.postDeltas = ResetPostDeltaVortex
	ps.resetIfOffline = resetTime
	ps.lastConnection = lastConnection
	ps.lastDeltaRemoval = lastConnection

	timeNowVortex = func() time.Time {
		return time.Date(2018, 12, 12, 12, 12, 12, 12, &time.Location{})
	}

	// And a set of stored deltas that occupy a given size in disk
	testhelpers.PopulateDeltas(dataDir, "entityKey", []testhelpers.FakeDeltaEntry{
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
	assert.True(t, storageSize < 10, "%v not smaller than 10", storageSize)
}

func newSender(t *testing.T, ctx *context, store *delta.Store, client http.Client) patchSender {
	pSender, err := newPatchSenderVortex("entityKey", agentKey, ctx, store, "user-agent", ctx.AgentIdentity, NewProvideIDs(newIncrementalRegister(), state.NewRegisterSM()), entity.NewKnownIDs(), client)
	require.NoError(t, err)
	return pSender
}
