// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package agent

import (
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/sirupsen/logrus"

	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi"
	"github.com/newrelic/infrastructure-agent/pkg/backend/state"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
)

var clog = log.WithComponent("IDProvider")

// ProvideIDs provides remote entity IDs.
// Waits for next retry if register endpoint status is not healthy.
type ProvideIDs func(agentIdn entity.Identity, entities []identityapi.RegisterEntity) ([]identityapi.RegisterEntityResponse, error)

type idProvider struct {
	client identityapi.IdentityRegisterClient
	state  state.RegisterSM
}

// NewProvideIDs creates a new remote entity IDs provider.
func NewProvideIDs(
	client identityapi.IdentityRegisterClient,
	sm state.RegisterSM,
) ProvideIDs {
	p := newIDProvider(client, sm)
	return p.provideIDs
}

func newIDProvider(client identityapi.IdentityRegisterClient, sm state.RegisterSM) *idProvider {
	return &idProvider{
		client: client,
		state:  sm,
	}
}

// provideIDs requests ID to register endpoint, waiting for retries on failures.
// Updates the entity Map and adds the entityId if we already have them.
func (p *idProvider) provideIDs(agentIdn entity.Identity, entities []identityapi.RegisterEntity) (ids []identityapi.RegisterEntityResponse, err error) {
retry:
	s := p.state.State()
	if s != state.RegisterHealthy {
		after := p.state.RetryAfter()
		clog.WithFields(logrus.Fields{
			"state":      s.String(),
			"retryAfter": after,
		}).Warn("unhealthy register state. Retry requested")
		time.Sleep(after)
		goto retry
	}

	var retryAfter time.Duration
	ids, retryAfter, err = p.client.Register(agentIdn.ID, entities)
	if err != nil {
		clog.WithFields(logrus.Fields{
			"agentID":    agentIdn,
			"retryAfter": retryAfter,
		}).Warn("cannot register entities, retry requested")
		if retryAfter > 0 {
			p.state.NextRetryAfter(retryAfter)
		} else {
			p.state.NextRetryWithBackoff()
		}
		return
	}
	return
}
