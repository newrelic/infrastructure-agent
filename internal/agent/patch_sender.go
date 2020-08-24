// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/entity"
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
	store            delta.Storage
	postDeltas       postDeltas
	lastSubmission   delta.LastSubmissionStore
	lastEntityID     delta.EntityIDPersist
	userAgent        string
	compactEnabled   bool
	compactThreshold uint64
	cfg              *config.Config
	context          AgentContext
	lastDeltaRemoval time.Time
	resetIfOffline   time.Duration
	agentIDProvide   id.Provide
	currentAgentID   entity.ID
}

type patchSender interface {
	Process() error
}

// Reference to the `time.Now()` function  that can be stubbed for unit testing
var timeNow = time.Now

var pslog = log.WithComponent("PatchSender")

// Reference to post delta function that can be stubbed for unit testing
type postDeltas func(entityKeys []string, isAgent bool, deltas ...*inventoryapi.RawDelta) (*inventoryapi.PostDeltaResponse, error)

func newPatchSender(entityKey string, context AgentContext, store delta.Storage, lastSubmission delta.LastSubmissionStore, lastEntityID delta.EntityIDPersist, userAgent string, agentIDProvide id.Provide, httpClient http2.Client) (patchSender, error) {
	if store == nil {
		return nil, fmt.Errorf("creating patch sender: delta store can't be nil")
	}
	if lastSubmission == nil {
		return nil, fmt.Errorf("creating patch sender: last submission store can't be nil")
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

	return &patchSenderIngest{
		entityKey:        entityKey,
		store:            store,
		lastSubmission:   lastSubmission,
		lastEntityID:     lastEntityID,
		postDeltas:       client.PostDeltas,
		context:          context,
		userAgent:        userAgent,
		compactEnabled:   context.Config().CompactEnabled,
		compactThreshold: context.Config().CompactThreshold,
		cfg:              context.Config(),
		resetIfOffline:   resetIfOffline,
		agentIDProvide:   agentIDProvide,
	}, err
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

	// We reset the deltas if the postDeltas fails after agent has been offline for > 24h
	lastSubmissionTimeExceeded := p.isLastSubmissionTimeExceeded(now)
	longTimeDisconnected := lastSubmissionTimeExceeded && p.lastDeltaRemoval.Add(p.resetIfOffline).Before(now)

	agentEntityIDChanged := p.agentEntityIDChanged()

	if longTimeDisconnected || agentEntityIDChanged {
		llog.WithField("offlineTime", p.resetIfOffline).
			WithField("agentEntityIDChanged", agentEntityIDChanged).
			Info("Removing inventory cache")

		// Removing the store for the entity would force the agent recreating a fresh Delta Store
		if err := p.store.RemoveEntity(p.entityKey); err != nil {
			llog.WithError(err).Warn("Could not remove inventory cache")
		}

		if agentEntityIDChanged {
			if err := p.lastEntityID.UpdateEntityID(p.agentIDProvide().ID); err != nil {
				llog.WithError(err).Warn("Failed to update inventory agent entityID")
			}
		}

		p.lastDeltaRemoval = now
		return fmt.Errorf("agent has to remove inventory cache")
	}

	if len(deltas) == 0 {
		llog.Debug("Patch sender found no deltas to send.")
		return nil
	}

	if !p.cfg.OfflineLoggingMode {
		if err = p.sendAllDeltas(deltas); err == nil && lastSubmissionTimeExceeded {
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

func (p *patchSenderIngest) sendAllDeltas(allDeltas []inventoryapi.RawDeltaBlock) error {
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

		if p.entityKey != "" {
			if err = p.lastSubmission.UpdateTime(timeNow()); err != nil {
				llog.WithError(err).Error("can't save submission time")
			}
		}

		if postDeltaResults.Reset == inventoryapi.ResetAll {
			reset = true
		} else {
			deltaStateResults := (*postDeltaResults).StateMap
			p.store.UpdateState(entityKey, deltas, &deltaStateResults)
		}
	}

	return nil
}

func (p *patchSenderIngest) isLastSubmissionTimeExceeded(now time.Time) bool {
	// Empty entity keys will be attached to agent entityKey so no need to reset.
	if p.entityKey == "" {
		return false
	}
	lastConn, err := p.lastSubmission.Time()
	if err != nil {
		pslog.WithField("entityKey", p.entityKey).
			WithError(err).
			Warn("failed when retrieve last submission time")
	}

	return lastConn.Add(p.resetIfOffline).Before(now)
}

func (p *patchSenderIngest) agentEntityIDChanged() bool {
	// Only check for entityID when is the agent sender.
	if p.entityKey != p.context.AgentIdentifier() {
		return false
	}

	lastEntityID, err := p.lastEntityID.GetEntityID()
	if err != nil {
		pslog.WithField("entityKey", p.entityKey).
			WithError(err).
			Warn("could not retrieve entityID")
	}

	currentAgentId := p.agentIDProvide().ID

	if lastEntityID == entity.EmptyID {
		err = p.lastEntityID.UpdateEntityID(currentAgentId)
		if err != nil {
			pslog.WithField("entityKey", p.entityKey).
				WithError(err).
				Warn("could not save entityID")
		}
		return false
	}

	return lastEntityID != p.agentIDProvide().ID
}
