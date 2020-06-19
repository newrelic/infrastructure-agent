// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/log"

	"github.com/newrelic/infrastructure-agent/internal/agent/delta"
	"github.com/newrelic/infrastructure-agent/internal/agent/id"
	http2 "github.com/newrelic/infrastructure-agent/pkg/backend/http"
	"github.com/newrelic/infrastructure-agent/pkg/backend/inventoryapi"
	"github.com/sirupsen/logrus"

	"github.com/newrelic/infrastructure-agent/pkg/config"
)

// patchSender collects the cached plugin deltas and submits them to the backend ingest service
type patchSenderIngest struct {
	entityKey        string
	store            *delta.Store
	postDeltas       postDeltas
	userAgent        string
	compactEnabled   bool
	compactThreshold uint64
	cfg              *config.Config
	context          AgentContext
	lastConnection   time.Time
	lastDeltaRemoval time.Time
	resetIfOffline   time.Duration
}

type patchSender interface {
	Process() error
}

// Reference to the `time.Now()` function  that can be stubbed for unit testing
var timeNow = time.Now

var pslog = log.WithComponent("PatchSender")

// Reference to post delta function that can be stubbed for unit testing
type postDeltas func(entityKeys []string, isAgent bool, deltas ...*inventoryapi.RawDelta) (*inventoryapi.PostDeltaResponse, error)

func newPatchSender(entityKey string, context AgentContext, store *delta.Store, userAgent string, agentIDProvide id.Provide, httpClient http2.Client) (patchSender, error) {
	if store == nil {
		pslog.WithField("entityKey", entityKey).Error("creating patch sender: delta store can't be nil")
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
		pslog.WithFields(logrus.Fields{
			"entityKey":   entityKey,
			"actualValue": context.Config().OfflineTimeToReset,
		}).WithError(err).Warn("configuration property offline_time_to_reset has an invalid format. Setting to '24h'")

		resetIfOffline, _ = time.ParseDuration("24h")
	}
	now := timeNow()
	return &patchSenderIngest{
		entityKey:        entityKey,
		store:            store,
		postDeltas:       client.PostDeltas,
		context:          context,
		userAgent:        userAgent,
		compactEnabled:   context.Config().CompactEnabled,
		compactThreshold: context.Config().CompactThreshold,
		cfg:              context.Config(),
		lastConnection:   now,
		lastDeltaRemoval: now,
		resetIfOffline:   resetIfOffline,
	}, nil
}

func (p *patchSenderIngest) Process() (err error) {
	llog := pslog.WithField("entityKey", p.entityKey)

	now := timeNow()

	// Note that caller will use a back-off on calling frequency
	// based on receiving error codes from this function
	deltas, err := p.store.ReadDeltas(p.entityKey)
	if err != nil {
		llog.WithError(err).Error("patch sender process reading deltas")
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

func (p *patchSenderIngest) sendAllDeltas(allDeltas []inventoryapi.RawDeltaBlock, currentTime time.Time) error {
	llog := pslog.WithField("entityKey", p.entityKey)

	// This variable means the entity these deltas represent is an agent

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

	reset := false

	defer func() {
		if reset {
			llog.Debug("Full Plugin Inventory Reset Requested.")
			p.store.ResetAllDeltas(entityKey)
			p.context.Reconnect() // Relaunching one-time harvesters to avoid losing the inventories after reset
			err := p.store.SaveState()
			if err != nil {
				llog.WithError(err).Error("error after resetting deltas while flushing inventory to cache")
			}
		}
	}()

	llog.WithField("numberOfBlocks", len(allDeltas)).Debug("Sending deltas divided in blocks.")
	for n, deltas := range allDeltas {
		llog.WithFieldsF(func() logrus.Fields {
			deltaJson, err := json.Marshal(deltas)
			if err == nil {
				return logrus.Fields{"blockNumber": n, "sizeBytes": deltas, "json": string(deltaJson)}
			}
			return logrus.Fields{"blockNumber": n, "sizeBytes": len(deltas), logrus.ErrorKey: err}
		}).Debug("Sending deltas block.")

		var postDeltaResults *inventoryapi.PostDeltaResponse
		var err error
		if postDeltaResults, err = p.postDeltas([]string{entityKey}, areAgentDeltas, deltas...); err != nil {
			llog.WithError(err).WithFields(logrus.Fields{
				"areAgentDeltas":   areAgentDeltas,
				"postDeltaResults": fmt.Sprintf("%+v", postDeltaResults),
			}).Error("couldn't post deltas")
			return err
		}

		p.lastConnection = currentTime
		if postDeltaResults.Reset == inventoryapi.ResetAll {
			reset = true
		} else {
			deltaStateResults := (*postDeltaResults).StateMap
			p.store.UpdateState(entityKey, deltas, &deltaStateResults)
		}
	}

	return nil
}
