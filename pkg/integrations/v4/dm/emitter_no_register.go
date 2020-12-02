package dm

import (
	"fmt"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/backend/http"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/entity/host"
	"github.com/newrelic/infrastructure-agent/pkg/fwrequest"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/legacy"
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

		logEntry := elog.WithField("action", "EmitV4DataSet")

		replaceEntityNameWithoutRegister := func(entity entity.Fields, entityRewrite data.EntityRewrites, idLookup host.IDLookup) error {

			newName := entityRewrite.Apply(entity.Name)

			agentShortName, err := idLookup.AgentShortEntityName()
			newName = http.ReplaceLocalhost(newName, agentShortName)

			if err != nil {
				return err
			}

			entity.Name = newName
			return nil
		}

		err := replaceEntityNameWithoutRegister(dataSet.Entity, entityRewrite, idLookup)
		if err != nil {
			return fmt.Errorf("error renaming entity: %s", err.Error())
		}

		integrationUser := definition.ExecutorConfig.User

		if len(dataSet.Inventory) > 0 {
			inventoryDataSet := legacy.BuildInventoryDataSet(
				logEntry, dataSet.Inventory, labels, integrationUser, integrationMetadata.Name,
				dataSet.Entity.Name)
			emitter.EmitInventory(inventoryDataSet, entity.Entity{
				Key: entity.Key(dataSet.Entity.Name),
			})
		}

		for _, event := range dataSet.Events {
			normalizedEvent := legacy.NormalizeEvent(elog, event, labels, integrationUser, dataSet.Entity.Name)

			if normalizedEvent != nil {
				emitter.EmitEvent(normalizedEvent, entity.Key(dataSet.Entity.Name))
			}
		}

		dmProcessor := IntegrationProcessor{
			IntegrationInterval:         definition.Interval,
			IntegrationLabels:           labels,
			IntegrationExtraAnnotations: extraAnnotations,
		}
		metricsSender.SendMetrics(dmProcessor.ProcessMetrics(dataSet.Metrics, dataSet.Common, dataSet.Entity))

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
