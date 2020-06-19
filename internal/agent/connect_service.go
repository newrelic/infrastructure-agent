// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package agent

import (
	"errors"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/log"

	"github.com/newrelic/infrastructure-agent/pkg/backend/backoff"
	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/helpers/fingerprint"
	"github.com/newrelic/infrastructure-agent/pkg/trace"
)

type identityConnectService struct {
	fingerprintHarvest fingerprint.Harvester
	lastFingerprint    fingerprint.Fingerprint
	client             identityapi.IdentityConnectClient
}

// ErrEmptyEntityID is returned when the entityID is empty.
var ErrEmptyEntityID = errors.New("the agentID provided is empty. make sure you have connected if this is not expected")

var logger = log.WithComponent("IdentityConnectService")

func NewIdentityConnectService(client identityapi.IdentityConnectClient, fingerprintHarvest fingerprint.Harvester) *identityConnectService {
	return &identityConnectService{
		fingerprintHarvest: fingerprintHarvest,
		client:             client,
	}
}

func (ic *identityConnectService) Connect() entity.Identity {
	var retryBO *backoff.Backoff

	for {
		f, err := ic.fingerprintHarvest.Harvest()
		if err != nil {
			logger.Warn("fingerprint harvest failed")
			time.Sleep(1 * time.Second)
			continue
		}

		trace.Connect("connect request with fingerprint: %+v", f)

		ids, retry, err := ic.client.Connect(f)

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

	var retryBO *backoff.Backoff
	for {
		trace.Connect("connect update request with fingerprint: %+v", f)
		retry, entityIdn, err := ic.client.ConnectUpdate(agentIdn, f)
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
	logger.WithField("state", state).Info("calling disconnect")

	if agentID.IsEmpty() {
		return ErrEmptyEntityID
	}

	if err := ic.client.Disconnect(agentID, state); err != nil {
		logger.WithError(err).Error("cannot disconnect")
		return err
	}

	return nil
}
