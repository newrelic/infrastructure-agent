// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package agent

import (
	goContext "context"
	"errors"
	"fmt"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent/instrumentation"
	"github.com/newrelic/infrastructure-agent/pkg/backend/backoff"
	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi"
	"github.com/newrelic/infrastructure-agent/pkg/config" //nolint:depguard
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/helpers/fingerprint"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

type identityConnectService struct {
	fingerprintHarvest fingerprint.Harvester
	lastFingerprint    fingerprint.Fingerprint
	client             identityapi.IdentityConnectClient
	metadataHarvester  identityapi.MetadataHarvester
}

// ErrEmptyEntityID is returned when the entityID is empty.
var ErrEmptyEntityID = errors.New("the agentID provided is empty. make sure you have connected if this is not expected")

var logger = log.WithComponent("IdentityConnectService")

//nolint:revive
func NewIdentityConnectService(client identityapi.IdentityConnectClient, fingerprintHarvest fingerprint.Harvester, metadataHarvester identityapi.MetadataHarvester) *identityConnectService {
	return &identityConnectService{
		fingerprintHarvest: fingerprintHarvest,
		client:             client,
		metadataHarvester:  metadataHarvester,
	}
}

func (ic *identityConnectService) Connect() entity.Identity {
	var retryBO *backoff.Backoff

	_, txn := instrumentation.SelfInstrumentation.StartTransaction(goContext.Background(), "agent.connect")
	defer txn.End()

	for {
		f, err := ic.fingerprintHarvest.Harvest()
		if err != nil {
			logger.Warn("fingerprint harvest failed")
			time.Sleep(1 * time.Second)
			continue
		}

		logger.WithField(config.TracesFieldName, config.FeatureTrace).Tracef("connect request with fingerprint: %+v", f)

		metatada, err := ic.metadataHarvester.Harvest()
		if err != nil {
			logger.WithError(err).Error("metadata harvest failed")
			time.Sleep(1 * time.Second)

			continue
		}

		ids, retry, err := ic.client.Connect(f, metatada)

		if !ids.ID.IsEmpty() {
			logger.
				WithField("agent-id", ids.ID).
				WithField("agent-guid", ids.GUID).
				Infof("connect got id")
			// save fingerprint for later (connect update)
			ic.lastFingerprint = f
			return ids
		}

		if retry.After > 0 {
			logger.WithField("retryAfter", retry.After).Debug("Connect retry requested.")
			retryBO = nil
			time.Sleep(retry.After)
			continue
		}

		if err != nil {
			logger.WithError(err).Warn("agent connect attempt failed")
		}

		if retryBO == nil {
			retryBO = backoff.NewDefaultBackoff()
		}
		retryBOAfter := retryBO.DurationWithMax(retry.MaxBackOff)
		logger.WithField("retryBackoffAfter", retryBOAfter).Debug("Connect backoff and retry requested.")
		time.Sleep(retryBOAfter)
	}
}

// ConnectUpdate will check for system fingerprint changes and will update it if it's the case.
// It returns the same ID provided as argument if there is an error
func (ic *identityConnectService) ConnectUpdate(agentIdn entity.Identity) (entityIdnOut entity.Identity, err error) {
	logger.Debug("Connect update process started.")
	_, txn := instrumentation.SelfInstrumentation.StartTransaction(goContext.Background(), "agent.connect_update")
	defer txn.End()

	if agentIdn.ID.IsEmpty() {
		logger.Warn(ErrEmptyEntityID.Error())
	}

	f, err := ic.fingerprintHarvest.Harvest()
	if err != nil {
		logger.WithError(err).Error("Failed to harvest fingerprint.")
		return agentIdn, err
	}
	logger.Debug("Fingerprint harvested successfully.")

	// lastFingerprint must have been set in the first connect
	// if it didn't change, just return the same agentID
	if ic.lastFingerprint.Equals(f) {
		logger.Debug("Fingerprint has not changed. Skipping connect update.")
		return agentIdn, nil
	}

	logger.Debug("Fingerprint has changed. Proceeding with connect update.")
	// fingerprint changed, so let's store it for the next round
	ic.lastFingerprint = f

	metadata, err := ic.metadataHarvester.Harvest()
	if err != nil {
		logger.WithError(err).Error("Failed to harvest metadata.")
		return agentIdn, fmt.Errorf("failed to harvest metadata: %w", err)
	}
	logger.Debug("Metadata harvested successfully.")

	var retryBO *backoff.Backoff
	for {
		logger.Debug("Entering connect update retry loop.")
		logger.WithField(config.TracesFieldName, config.FeatureTrace).Tracef("connect update request with fingerprint: %+v", f)

		retry, updatedEntityIdn, err := ic.client.ConnectUpdate(agentIdn, f, metadata)

		// This handles the case where the update call itself is successful, but the server asks us to retry.
		if err == nil && retry.After > 0 {
			logger.WithField("retryAfter", retry.After).Debug("Connect update API requested a retry after a specific time.")
			retryBO = nil // Reset backoff timer on explicit retry-after from server
			time.Sleep(retry.After)
			continue
		}

		// This handles cases where the update call failed (e.g., network error, 5xx status)
		if err != nil {
			logger.WithError(err).Warn("Agent connect update attempt failed.")

			if retryBO == nil {
				logger.Debug("Initializing new backoff timer.")
				retryBO = backoff.NewDefaultBackoff()
			}
			retryBOAfter := retryBO.DurationWithMax(retry.MaxBackOff)
			logger.WithField("retryBackoffAfter", retryBOAfter).Debug("Connect update backoff and retry requested.")
			time.Sleep(retryBOAfter)
			continue
		}

		// If we reach here, err is nil and retry.After is not > 0, so the call was successful.
		logger.WithField("entityID", updatedEntityIdn.ID).Debug("Connect update process finished successfully.")
		return updatedEntityIdn, nil
	}
}

// Disconnect is used to signal the backend that the agent will stop.
func (ic *identityConnectService) Disconnect(agentID entity.ID, state identityapi.DisconnectReason) error {
	_, txn := instrumentation.SelfInstrumentation.StartTransaction(goContext.Background(), "agent.disconnect")
	defer txn.End()

	logger.WithField("state", state).Info("calling disconnect")

	if agentID.IsEmpty() {
		return ErrEmptyEntityID
	}

	if err := ic.client.Disconnect(agentID, state); err != nil {
		logger.WithError(err).Error("cannot disconnect")
		return err
	}

	logger.WithField("state", state).Info("disconnect call finished")
	return nil
}
