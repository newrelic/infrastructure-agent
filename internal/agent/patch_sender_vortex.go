// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	wlog "github.com/newrelic/infrastructure-agent/pkg/log"

	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/entity"

	"github.com/newrelic/infrastructure-agent/internal/agent/delta"
	"github.com/newrelic/infrastructure-agent/internal/agent/id"
	http2 "github.com/newrelic/infrastructure-agent/pkg/backend/http"
	"github.com/newrelic/infrastructure-agent/pkg/backend/inventoryapi"
	"github.com/sirupsen/logrus"
)

type patchSenderVortex struct {
	entityKey        string
	agentKey         string
	store            *delta.Store
	postDeltas       postDeltasVortex
	userAgent        string
	compactEnabled   bool
	compactThreshold uint64
	cfg              *config.Config
	context          AgentContext
	lastConnection   time.Time
	lastDeltaRemoval time.Time
	resetIfOffline   time.Duration
	provideIDs       ProvideIDs
	entityMap        entity.KnownIDs
	agentID          id.Provide
}

// Reference to the `time.Now()` function  that can be stubbed for unit testing
var timeNowVortex = time.Now

var psvlog = wlog.WithComponent("PatchSenderVortex")

// Reference to post delta function that can be stubbed for unit testing
type postDeltasVortex func(entityID entity.ID, entityKeys []string, isAgent bool, deltas ...*inventoryapi.RawDelta) (*inventoryapi.PostDeltaResponse, error)

func newPatchSenderVortex(entityKey, agentKey string, context AgentContext, store *delta.Store, userAgent string, agentIDProvide id.Provide, provideIDs ProvideIDs, entityMap entity.KnownIDs, httpClient http2.Client) (patchSender, error) {
	if store == nil {
		psvlog.WithField("entityKey", entityKey).Error("creating patch sender: delta store can't be nil")
		panic("creating patch sender: delta store can't be nil")
	}
	inventoryURL := fmt.Sprintf("%s/%s", context.Config().CollectorURL,
		strings.TrimPrefix(context.Config().InventoryIngestEndpoint, "/"))
	if os.Getenv("DEV_INVENTORY_INGEST_URL") != "" {
		inventoryURL = os.Getenv("DEV_INVENTORY_INGEST_URL")
	}
	inventoryURL = strings.TrimSuffix(inventoryURL, "/")
	client, err := inventoryapi.NewIngestClient(
		inventoryURL,
		context.Config().License,
		userAgent,
		context.Config().PayloadCompressionLevel,
		context.AgentIdentifier(),
		agentIDProvide,
		context.Config().ConnectEnabled,
		httpClient,
	)
	if err != nil {
		return nil, err
	}
	resetIfOffline, err := time.ParseDuration(context.Config().OfflineTimeToReset)
	if err != nil {
		psvlog.WithFields(logrus.Fields{
			"entityKey":   entityKey,
			"actualValue": context.Config().OfflineTimeToReset,
		}).WithError(err).Warn("configuration property offline_time_to_reset has an invalid format. Setting to '24h'")

		resetIfOffline, _ = time.ParseDuration("24h")
	}
	now := timeNowVortex()
	return &patchSenderVortex{
		entityKey:        entityKey,
		agentKey:         agentKey,
		store:            store,
		postDeltas:       client.PostDeltasVortex,
		context:          context,
		userAgent:        userAgent,
		compactEnabled:   context.Config().CompactEnabled,
		compactThreshold: context.Config().CompactThreshold,
		cfg:              context.Config(),
		lastConnection:   now,
		lastDeltaRemoval: now,
		resetIfOffline:   resetIfOffline,
		provideIDs:       provideIDs,
		entityMap:        entityMap,
		agentID:          agentIDProvide,
	}, nil
}

// Add workers to register entities whose ID we don't have in our map already.

func (p *patchSenderVortex) Process() (err error) {
	llog := psvlog.WithField("entityKey", p.entityKey)

	now := timeNowVortex()

	// Note that caller will use a back-off on calling frequency
	// based on receiving error codes from this function
	deltas, err := p.store.ReadDeltas(p.entityKey)
	if err != nil {
		llog.WithError(err).Error("Patch sender process reading deltas.")
		return
	}
	if len(deltas) == 0 {
		llog.Debug("Patch sender found no deltas to send.")
		// Update the lastConnection time to stop the `agent has been offline`
		// error from being triggered
		p.lastConnection = now
		return nil
	}

	llog.WithFieldsF(func() logrus.Fields {
		return logrus.Fields{
			"lastConnection": p.lastConnection,
			"currentTime":    now,
		}
	}).Debug("Prepare to process deltas.")

	// We reset the deltas if the postDeltas fails after agent has been offline for > 24h
	longTimeDisconnected := p.lastConnection.Add(p.resetIfOffline).Before(now)
	if longTimeDisconnected && p.lastDeltaRemoval.Add(p.resetIfOffline).Before(now) {
		llog.WithField("offlineTime", p.resetIfOffline).
			Info("agent has been offline for too long. Recreating delta store")

		// Removing the store for the entity would force the agent recreating a fresh Delta Store
		if err := p.store.RemoveEntity(p.entityKey); err != nil {
			llog.WithError(err).Warn("removing deltas")
		}
		p.lastDeltaRemoval = now
		return fmt.Errorf("agent has been offline for %v min. Need to reset delta store", p.resetIfOffline)
	}
	if !p.cfg.OfflineLoggingMode {
		if err = p.sendAllDeltas(deltas, now); err == nil && longTimeDisconnected {
			// If the agent has been long time disconnected, we re-run the reconnecting plugins
			p.context.Reconnect()
		}
	} else {
		llog.WithField("numberOfDeltas", len(deltas)).Info("suppressed PostDeltas")
	}

	if p.compactEnabled {
		if cerr := p.store.CompactStorage(p.entityKey, p.compactThreshold); cerr != nil {
			llog.WithError(cerr).WithField("compactThreshold", p.compactThreshold).
				Error("compaction error")

		}
	}

	return
}

// Nice2Have: blocking function. Migrate agent to asynchronous architecture
// sendAllDeltas returns error only on data submission failures
func (p *patchSenderVortex) sendAllDeltas(allDeltas []inventoryapi.RawDeltaBlock, currentTime time.Time) error {
	llog := pslog.WithField("entityKey", p.entityKey)

	var areAgentDeltas bool
	var entityKey string
	// Empty entity Key belong to the Agent
	if p.entityKey == "" {
		areAgentDeltas = true
		entityKey = p.context.AgentIdentifier()
	} else {
		areAgentDeltas = p.entityKey == p.context.AgentIdentifier()
		entityKey = p.entityKey
	}

	// Set to agent ID and overwrite in case it's not agent delta
	entityID := p.agentID().ID
	if !areAgentDeltas {
		// If the entityKey is void we assume it's inventory from the agent. <- As seen in the sendAllDeltas function first if.
		if p.entityKey != p.agentKey && p.entityKey != "" {
			var found = false
			entityID, found = p.entityMap.Get(entity.Key(p.entityKey))
			if !found {
				e := identityapi.NewRegisterEntity(entity.Key(p.entityKey))
				idRes, err := p.provideIDs(p.agentID(), []identityapi.RegisterEntity{e})
				// caller expect to handle connection errors, not register ones, so don't return these
				if err != nil {
					llog.WithError(err).Error("register error for inventory")
					return nil
				}
				if len(idRes) != 1 {
					llog.WithField("returnedID", idRes).
						Error("register for inventory entity did not return single result")
					return nil
				}
				entityID = idRes[0].ID
			}
		}
	}

	reset, err := p.sendDeltas(entityID, llog, allDeltas, areAgentDeltas, currentTime)
	if err != nil {
		llog.WithError(err).Error("error sending deltas ")
	}

	if reset {
		llog.Debug("Full Plugin Inventory Reset Requested.")
		p.store.ResetAllDeltas(entityKey)
		p.context.Reconnect() // Relaunching one-time harvesters to avoid losing the inventories after reset
		err := p.store.SaveState()
		if err != nil {
			llog.WithError(err).Error("error after reset deltas while flushing inventory to cache")
		}
	}

	return err
}

func (p *patchSenderVortex) sendDeltas(entityID entity.ID, llog wlog.Entry, allDeltas []inventoryapi.RawDeltaBlock, areAgentDeltas bool, currentTime time.Time) (reset bool, err error) {
	llog.WithField("numberOfBlocks", len(allDeltas)).Debug("Sending deltas divided in blocks.")
	for n, deltas := range allDeltas {
		llog.WithFieldsF(func() logrus.Fields {
			deltaJson, err := json.Marshal(deltas)
			if err == nil {
				return logrus.Fields{"blockNumber": n, "sizeBytes": deltas, "json": deltaJson}
			}
			return logrus.Fields{"blockNumber": n, "sizeBytes": deltas, logrus.ErrorKey: err}
		}).Debug("Sending deltas block.")

		var postDeltaResults *inventoryapi.PostDeltaResponse
		if postDeltaResults, err = p.postDeltas(entityID, []string{p.entityKey}, areAgentDeltas, deltas...); err != nil {
			llog.WithError(err).WithFields(logrus.Fields{
				"entityID":         entityID,
				"areAgentDeltas":   areAgentDeltas,
				"postDeltaResults": fmt.Sprintf("%+v", postDeltaResults),
			}).Error("couldn't post deltas")
			return
		}

		p.lastConnection = currentTime
		if postDeltaResults.Reset == inventoryapi.ResetAll {
			reset = true
		} else {
			deltaStateResults := (*postDeltaResults).StateMap
			p.store.UpdateState(p.entityKey, deltas, &deltaStateResults)
		}
	}
	return
}
