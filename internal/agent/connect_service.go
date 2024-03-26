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
func (ic *identityConnectService) ConnectUpdate(agentIdn entity.Identity) (entityIdn entity.Identity, err error) {
	_, txn := instrumentation.SelfInstrumentation.StartTransaction(goContext.Background(), "agent.connect_update")
	defer txn.End()

	if agentIdn.ID.IsEmpty() {
		logger.Warn(ErrEmptyEntityID.Error())
	}

	f, err := ic.fingerprintHarvest.Harvest()
	if err != nil {
		return agentIdn, err
	}

	// lastFingerprint must have been set in the first connect
	// if it didn't change, just return the same agentID
	if ic.lastFingerprint.Equals(f) {
		return agentIdn, nil
	}

	// fingerprint changed, so let's store it for the next round
	ic.lastFingerprint = f

	metatada, err := ic.metadataHarvester.Harvest()
	if err != nil {
		return agentIdn, fmt.Errorf("failed to harvest metadata: %w", err)
	}

	var retryBO *backoff.Backoff
	for {
		logger.WithField(config.TracesFieldName, config.FeatureTrace).Tracef("connect update request with fingerprint: %+v", f)
		retry, entityIdn, err := ic.client.ConnectUpdate(agentIdn, f, metatada)
		if retry.After > 0 {
			logger.WithField("retryAfter", retry.After).Debug("Connect update retry requested.")
			retryBO = nil
			time.Sleep(retry.After)
			continue
		}

		if err != nil {
			logger.WithError(err).Warn("agent connect update attempt failed")

			if retryBO == nil {
				retryBO = backoff.NewDefaultBackoff()
			}
			retryBOAfter := retryBO.DurationWithMax(retry.MaxBackOff)
			logger.WithField("retryBackoffAfter", retryBOAfter).Debug("Connect update backoff and retry requested.")
			time.Sleep(retryBOAfter)
			continue
		}
		return entityIdn, nil
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
