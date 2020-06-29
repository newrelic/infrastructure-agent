// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package emitter

import (
	"encoding/json"
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/legacy"

	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/handler"
	"github.com/newrelic/infrastructure-agent/internal/feature_flags"
	"github.com/newrelic/infrastructure-agent/pkg/backend/http"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/dm"
)

func (e *Legacy) EmitV4(
	metadata integration.Definition,
	extraLabels data.Map,
	entityRewrite []data.EntityRewrite,
	integrationData protocol.DataV4) error {
	var emitErrs []error

	pluginId := metadata.PluginID(integrationData.Integration.Name)
	plugin := agent.NewExternalPluginCommon(pluginId, e.Context, metadata.Name)

	labels, extraAnnotations := metadata.LabelsAndExtraAnnotations(extraLabels)

	var err error
	for _, dataset := range integrationData.DataSets {
		if err = emitV4DataSet(
			e.Context.IDLookup(),
			e.MetricsSender,
			&plugin,
			metadata,
			integrationData.Integration,
			dataset,
			labels,
			extraAnnotations,
			entityRewrite,
		); err != nil {
			emitErrs = append(emitErrs, err)
		}
	}

	return composeEmitError(emitErrs, len(integrationData.DataSets))
}

func emitV4DataSet(
	idLookup agent.IDLookup,
	metricsSender dm.MetricsSender,
	emitter agent.PluginEmitter,
	metadata integration.Definition,
	integrationMetadata protocol.IntegrationMetadata,
	dataSet protocol.Dataset,
	labels map[string]string,
	extraAnnotations map[string]string,
	entityRewrite []data.EntityRewrite) error {
	logEntry := elog.WithField("action", "EmitV4DataSet")

	err := replaceEntityName(dataSet.Entity, entityRewrite, idLookup)
	if err != nil {
		return fmt.Errorf("error renaming entity: %s", err.Error())
	}

	integrationUser := metadata.ExecutorConfig.User

	if len(dataSet.Inventory) > 0 {
		inventoryDataSet := legacy.BuildInventoryDataSet(
			logEntry, dataSet.Inventory, labels, integrationUser, integrationMetadata.Name,
			dataSet.Entity.Name)
		emitter.EmitInventory(inventoryDataSet, dataSet.Entity.Name)
	}

	for _, event := range dataSet.Events {
		normalizedEvent := legacy.NormalizeEvent(elog, event, labels, integrationUser, dataSet.Entity.Name)

		if normalizedEvent != nil {
			emitter.EmitEvent(normalizedEvent, entity.Key(dataSet.Entity.Name))
		}
	}

	dmProcessor := dm.IntegrationProcessor{
		IntegrationInterval:         metadata.Interval,
		IntegrationLabels:           labels,
		IntegrationExtraAnnotations: extraAnnotations,
	}
	metricsSender.SendMetrics(dmProcessor.ProcessMetrics(dataSet.Metrics, dataSet.Common, dataSet.Entity))

	return nil
}

// Replace entity name by applying entity rewrites and replacing loopback
func replaceEntityName(entity protocol.Entity, entityRewrite []data.EntityRewrite, idLookup agent.IDLookup) error {
	newName := legacy.ApplyEntityRewrite(entity.Name, entityRewrite)

	agentShortName, err := idLookup.AgentShortEntityName()
	newName = http.ReplaceLocalhost(newName, agentShortName)

	if err != nil {
		return err
	}

	entity.Name = newName
	return nil
}

// ParsePayloadV4 parses a string containing a JSON payload with the format of our
// SDK for v4 protocol which uses dimensional metrics.
func ParsePayloadV4(raw []byte, ffManager feature_flags.Retriever) (dataV4 protocol.DataV4, err error) {
	if len(raw) == 0 {
		err = NoContentToParseErr
		return
	}

	if enabled, ok := ffManager.GetFeatureFlag(handler.FlagProtocolV4); !ok || !enabled {
		err = ProtocolV4NotEnabledErr
		return
	}

	err = json.Unmarshal(raw, &dataV4)
	return
}
