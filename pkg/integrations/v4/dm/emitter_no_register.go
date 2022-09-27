// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package dm

import (
	"context"

	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/fwrequest"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
)

// emitDatasetWithEmptyEntity will emit the dataset with an empty entity and entity will be created in the backend
// through entity synthesis.
func (e *emitter) emitDatasetWithEmptyEntity(intMetadata protocol.IntegrationMetadata, reqMetadata fwrequest.FwRequestMeta, dataSet protocol.Dataset) {
	agentVersion := e.agentContext.Version()
	e.emitDataset(fwrequest.NewEntityFwRequest(dataSet, entity.EmptyID, reqMetadata, intMetadata, agentVersion))
}

// emitDatasetForAgent will emit a dataset fot the infra-agent.
func (e *emitter) emitDatasetForAgent(ctx context.Context, intMetadata protocol.IntegrationMetadata, reqMetadata fwrequest.FwRequestMeta, dataSet protocol.Dataset) {
	agentVersion := e.agentContext.Version()
	eID := e.agentContext.Identity().ID

	select {
	case <-ctx.Done():
		return
	case e.reqsRegisteredQueue <- fwrequest.NewEntityFwRequest(dataSet, eID, reqMetadata, intMetadata, agentVersion):
	}
}
