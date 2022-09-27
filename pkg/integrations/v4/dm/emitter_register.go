// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package dm

import (
	"context"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/fwrequest"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
)

func (e *emitter) runReqsRegisteredConsumer(ctx context.Context) {
	for {
		select {
		case _ = <-ctx.Done():
			return

		case eReq := <-e.reqsRegisteredQueue:
			e.processEntityFwRequest(eReq)
		}
	}
}

func (e *emitter) processEntityFwRequest(r fwrequest.EntityFwRequest) {
	// rewrites processing
	agentShortName, err := e.agentContext.IDLookup().AgentShortEntityName()
	if err != nil {
		elog.
			WithError(err).
			WithField("integration", r.Definition.Name).
			Errorf("cannot determine agent short name")
	}
	replaceEntityName(r.Data.Entity, r.EntityRewrite, agentShortName)

	key, err := r.Data.Entity.Key()
	if err != nil {
		elog.
			WithError(err).
			WithField("integration", r.Definition.Name).
			Errorf("cannot determine entity")
	} else {
		e.idCache.CleanOld()
		e.idCache.Put(key, r.ID())
	}
	e.emitDataset(r)
}

func (e *emitter) processDatasetRegister(ctx context.Context, intMetadata protocol.IntegrationMetadata, reqMetadata fwrequest.FwRequestMeta, dataSet protocol.Dataset) {

	agentVersion := e.agentContext.Version()
	eKey, err := dataSet.Entity.ResolveUniqueEntityKey(e.agentContext.EntityKey(), e.agentContext.IDLookup(), reqMetadata.EntityRewrite, 4)
	if err != nil {
		elog.
			WithError(err).
			WithField("integration", reqMetadata.Definition.Name).
			Errorf("couldn't determine a unique entity Key")
		return
	}

	eID, found := e.idCache.Get(eKey)
	if found {
		select {
		case <-ctx.Done():
			return

		case e.reqsRegisteredQueue <- fwrequest.NewEntityFwRequest(dataSet, eID, reqMetadata, intMetadata, agentVersion):
		}
		return
	}

	select {
	case <-ctx.Done():
		return

	case e.reqsToRegisterQueue <- fwrequest.NewEntityFwRequest(dataSet, entity.EmptyID, reqMetadata, intMetadata, agentVersion):
	}
}
