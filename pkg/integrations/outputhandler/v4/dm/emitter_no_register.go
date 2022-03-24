package dm

import (
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/fwrequest"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/outputhandler/v4/protocol"
)

func (e *emitter) processDatasetNoRegister(intMetadata protocol.IntegrationMetadata, reqMetadata fwrequest.FwRequestMeta, dataSet protocol.Dataset) {
	agentVersion := e.agentContext.Version()
	e.emitDataset(fwrequest.NewEntityFwRequest(dataSet, entity.EmptyID, reqMetadata, intMetadata, agentVersion))
}
