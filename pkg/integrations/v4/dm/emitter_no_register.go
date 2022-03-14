package dm

import (
	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/entity/host"
	"github.com/newrelic/infrastructure-agent/pkg/fwrequest"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
)

type nonRegisterEmitter struct {
	metricsSender MetricsSender
	agentContext  agent.AgentContext
}

func NewNonRegisterEmitter(agentContext agent.AgentContext, dmSender MetricsSender) Emitter {
	return &nonRegisterEmitter{
		agentContext:  agentContext,
		metricsSender: dmSender,
	}
}

func (e *emitter) processDatasetNoRegister(intMetadata protocol.IntegrationMetadata, reqMetadata fwrequest.FwRequestMeta, dataSet protocol.Dataset) {
	agentVersion := e.agentContext.Version()
	e.emitDataset(fwrequest.NewEntityFwRequest(dataSet, entity.EmptyID, reqMetadata, intMetadata, agentVersion))
}

func (e *nonRegisterEmitter) Send(dto fwrequest.FwRequest) {
	entityRewrite := dto.EntityRewrite
	integrationData := dto.Data

	var emitErrs []error

	plugin := agent.NewExternalPluginCommon(dto.PluginID(), e.agentContext, dto.Definition.Name)

	labels, extraAnnotations := dto.LabelsAndExtraAnnotations()

	var err error

	emitV4DataSet := func(
		idLookup host.IDLookup,
		metricsSender MetricsSender,
		emitter agent.PluginEmitter,
		definition integration.Definition,
		integrationMetadata protocol.IntegrationMetadata,
		dataSet protocol.Dataset,
		labels map[string]string,
		extraAnnotations map[string]string,
		entityRewrites data.EntityRewrites) error {

		emitInventory(emitter, definition, integrationMetadata, entity.EmptyID, dataSet, labels)

		emitEvent(emitter, definition, dataSet, labels, extraAnnotations, entity.EmptyID)

		emitMetrics(e.metricsSender, definition, dataSet, extraAnnotations, labels)

		return nil
	}

	for _, dataset := range integrationData.DataSets {
		if err = emitV4DataSet(
			e.agentContext.IDLookup(),
			e.metricsSender,
			&plugin,
			dto.Definition,
			integrationData.Integration,
			dataset,
			labels,
			extraAnnotations,
			entityRewrite,
		); err != nil {
			emitErrs = append(emitErrs, err)
		}
	}

	// TODO error handling
	composedError := composeEmitError(emitErrs, len(integrationData.DataSets))
	if composedError != nil {
		elog.Error(composedError.Error())
	}
}
