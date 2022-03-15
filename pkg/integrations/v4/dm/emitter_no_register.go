package dm

import (
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/fwrequest"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
)

func (e *emitter) processDatasetNoRegister(intMetadata protocol.IntegrationMetadata, reqMetadata fwrequest.FwRequestMeta, dataSet protocol.Dataset) {
	agentVersion := e.agentContext.Version()
	e.emitDataset(fwrequest.NewDatasetFwRequest(dataSet, entity.EmptyID, reqMetadata, intMetadata, agentVersion))
}
