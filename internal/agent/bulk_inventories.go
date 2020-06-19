// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package agent

import (
	"github.com/newrelic/infrastructure-agent/internal/agent/bulk"
	"github.com/newrelic/infrastructure-agent/internal/agent/delta"
	"github.com/newrelic/infrastructure-agent/pkg/backend/inventoryapi"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

var blog = log.WithComponent("BulkPatchSender")

// Inventories processes inventories from multiple entities and splits them if they are too big for the limited sizes
// imposed by the inventory ingest API
type Inventories struct {
	store *delta.Store

	buffer bulk.Buffer
	send   backendPost

	agentIdentifier  string
	compactEnabled   bool
	compactThreshold uint64

	ctx AgentContext

	// reference to the inventories from agent
	inventories *map[string]*inventory
}

// backendPost defines the prototype for a function that submits the bulk of PostDeltaBody objects to the backend
type backendPost func(reqs []inventoryapi.PostDeltaBody) ([]inventoryapi.BulkDeltaResponse, error)

// NewInventories instantiates and returns a new Inventories object given the configuration passed as arguments
func NewInventories(store *delta.Store, ctx AgentContext, client *inventoryapi.IngestClient, inventories *map[string]*inventory,
	agentIdentifier string, compactEnabled bool, compactThreshold uint64, maxDataSize int) Inventories {

	b := bulk.NewBuffer(maxDataSize)
	b.Clear()
	return Inventories{
		store: store,

		buffer: b,

		send: client.PostDeltasBulk,

		agentIdentifier:  agentIdentifier,
		compactEnabled:   compactEnabled,
		compactThreshold: compactThreshold,
		ctx:              ctx,
		inventories:      inventories,
	}
}

// BulkPatchProcess processes the deltas from all the entities in the Delta Store and submits them to the ingest service,
// splitting them if necessary into multiple requests.
// This function is not thread-safe.
// This function is blocking.
func (i *Inventories) BulkPatchProcess() error {
	saveStoreState := false
	defer func() {
		i.buffer.Clear()
		if saveStoreState {
			i.store.SaveState()
		}
	}()

	for entityKey := range *i.inventories {
		deltaBodies, err := i.processEntity(entityKey)
		if err != nil {
			blog.WithError(err).WithField("entityKey", entityKey).Warn("reading deltas")
			continue
		}
		if len(deltaBodies) == 0 {
			blog.WithField("entityKey", entityKey).Debug("Patch sender found no deltas to send.")
			continue
		}
		saveStoreState = true
		for _, deltaBody := range deltaBodies {
			// if the delta body can't be buffered because there is no space, previous bulks are sent to leave space
			if err := i.buffer.Add(entity.Key(entityKey), deltaBody); err != nil {
				i.submitBulks()
				i.buffer.Clear()
				// If the payload can't be sent after empty the buffer because it's too big, abort and return an error
				if err := i.buffer.Add(entity.Key(entityKey), deltaBody); err != nil {
					return err
				}
			}
		}
		if i.compactEnabled {
			if err := i.store.CompactStorage(entityKey, i.compactThreshold); err != nil {
				blog.WithField("entityKey", entityKey).WithError(err).Error("compaction error")
			}
		}
	}
	if i.buffer.Entries() > 0 {
		i.submitBulks()
	}

	return nil
}

// processEntity processes the deltas from the given entity and creates the `PostDeltabody` objects to be sent to the
// ingest service.
// This function is Blocking.
func (i Inventories) processEntity(entityKey string) ([]inventoryapi.PostDeltaBody, error) {
	deltaBlocks, err := i.store.ReadDeltas(entityKey)
	if err != nil {
		return nil, err
	}
	if len(deltaBlocks) == 0 {
		return nil, nil
	}

	isAgent := entityKey == i.agentIdentifier

	bodies := make([]inventoryapi.PostDeltaBody, 0, len(deltaBlocks))
	for _, readDelta := range deltaBlocks {
		if len(deltaBlocks) > 0 {
			body := inventoryapi.PostDeltaBody{
				ExternalKeys: []string{entityKey},
				IsAgent:      &isAgent,
				Deltas:       readDelta,
			}
			bodies = append(bodies, body)
		}
	}

	return bodies, nil
}

// This function is Blocking.
func (i Inventories) submitBulks() {
	defer i.buffer.Clear()

	// Submits to the backend the list of post deltas
	responses, err := i.send(i.buffer.AsSlice())
	if err != nil {
		blog.WithError(err).Error("sending inventory")
		return
	}
	// Then analises the individual response for each entity
	for _, r := range responses {
		entityKey := r.EntityKeys[0]
		if r.Reset == inventoryapi.ResetAll {
			blog.Debug("Full Plugin Data Reset Requested.")
			i.store.ResetAllDeltas(entityKey)
			i.ctx.Reconnect() // Relaunching one-time harvesters to avoid losing the inventories after reset
		} else {
			deltaStateResults := r.StateMap
			i.store.UpdateState(r.EntityKeys[0], i.buffer.Get(entity.Key(entityKey)).Deltas, &deltaStateResults)
		}
	}
}
