// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package dm

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/handler"
	"github.com/newrelic/infrastructure-agent/internal/feature_flags"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/backend/http"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/legacy"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

var (
	// Errors
	ProtocolV4NotEnabledErr = errors.New("integration protocol version 4 is not enabled")
	NoContentToParseErr     = errors.New("no content to parse")

	// internal
	elog = log.WithComponent("DimensionalMetricsEmitter")
)

const (
	nrEntityId = "nr.entity.id"
)

type Agent interface {
	GetContext() agent.AgentContext
}

type emitter struct {
	ffRetriever   feature_flags.Retriever
	metricsSender MetricsSender
	agentContext  agent.AgentContext
	idProvider    idProviderInterface
}

type Emitter interface {
	Send(
		metadata integration.Definition,
		extraLabels data.Map,
		entityRewrite []data.EntityRewrite,
		integrationJSON []byte) error
}

func NewEmitter(
	agentContext agent.AgentContext,
	dmSender MetricsSender,
	ffRetriever feature_flags.Retriever,
	idProvider idProviderInterface) Emitter {

	return &emitter{
		agentContext:  agentContext,
		metricsSender: dmSender,
		ffRetriever:   ffRetriever,
		idProvider:    idProvider,
	}
}

func (e *emitter) Send(
	metadata integration.Definition,
	extraLabels data.Map,
	entityRewrite []data.EntityRewrite,
	integrationJSON []byte) error {

	pluginDataV4, err := ParsePayloadV4(integrationJSON, e.ffRetriever)
	if err != nil {
		elog.WithError(err).WithField("output", string(integrationJSON)).Warn("can't parse v4 integration output")
		return err
	}

	return e.process(metadata, extraLabels, entityRewrite, pluginDataV4)
}

func (e *emitter) process(
	metadata integration.Definition,
	extraLabels data.Map,
	entityRewrite []data.EntityRewrite,
	integrationData protocol.DataV4) (err error) {

	pluginId := metadata.PluginID(integrationData.Integration.Name)
	plugin := agent.NewExternalPluginCommon(pluginId, e.agentContext, metadata.Name)
	labels, extraAnnotations := metadata.LabelsAndExtraAnnotations(extraLabels)

	var entities []protocol.Entity
	datasetsByEntityName := make(map[string]protocol.Dataset, len(integrationData.DataSets))
	// Collect All entities
	for i := range integrationData.DataSets {
		entities = append(entities, integrationData.DataSets[i].Entity)
		datasetsByEntityName[integrationData.DataSets[i].Entity.Name] = integrationData.DataSets[i]
	}

	agentShortName, err := e.agentContext.IDLookup().AgentShortEntityName()
	if err != nil {
		return wrapError(fmt.Errorf("error renaming entity: %s", err.Error()), len(integrationData.DataSets))
	}

	var emitErrs []error
	processEntityDataset := func(dataset protocol.Dataset, entityID entity.ID) {
		// for dataset.Entity call emitV4DataSet function with entity ID

		dataset.Common.Attributes[nrEntityId] = entityID.String()
		replaceEntityName(dataset.Entity, entityRewrite, agentShortName)

		emitInventory(
			&plugin,
			metadata,
			integrationData.Integration,
			entityID,
			dataset,
			labels,
		)

		emitEvent(
			&plugin,
			metadata,
			dataset,
			labels,
		)

		dmProcessor := IntegrationProcessor{
			IntegrationInterval:         metadata.Interval,
			IntegrationLabels:           labels,
			IntegrationExtraAnnotations: extraAnnotations,
		}

		metrics := dmProcessor.ProcessMetrics(dataset.Metrics, dataset.Common, dataset.Entity)
		if err := e.metricsSender.SendMetricsWithCommonAttributes(dataset.Common, metrics); err != nil {
			// TODO error handling
		}
	}

	registeredEntities, unregisteredEntitiesWithWait := e.RegisterEntities(entities)

	for entityName, entityID := range registeredEntities {
		processEntityDataset(datasetsByEntityName[entityName], entityID)
	}

	if len(unregisteredEntitiesWithWait.entities) == 0 {
		return composeEmitError(emitErrs, len(integrationData.DataSets))
	}

	unregisteredEntitiesWithWait.waitGroup.Wait()
	entitiesToReRegister := make([]protocol.Entity, 0)

	for i := range unregisteredEntitiesWithWait.entities {
		if unregisteredEntitiesWithWait.entities[i].Reason != reasonEntityError {
			entitiesToReRegister = append(entitiesToReRegister, unregisteredEntitiesWithWait.entities[i].Entity)
		} else {
			emitErrs = append(emitErrs, fmt.Errorf(
				"entity with name '%s' was not registered in the backend, err '%v'",
				unregisteredEntitiesWithWait.entities[i].Entity.Name, unregisteredEntitiesWithWait.entities[i].Err))
		}
	}

	if len(entitiesToReRegister) == 0 {
		return composeEmitError(emitErrs, len(integrationData.DataSets))
	}

	registeredEntities, unregisteredEntitiesWithWait = e.RegisterEntities(entitiesToReRegister)

	for entityName, entityID := range registeredEntities {
		processEntityDataset(datasetsByEntityName[entityName], entityID)
	}

	if len(unregisteredEntitiesWithWait.entities) == 0 {
		return composeEmitError(emitErrs, len(integrationData.DataSets))
	}

	for i := range unregisteredEntitiesWithWait.entities {
		emitErrs = append(emitErrs, fmt.Errorf(
			"entity with name '%s' was not registered in the backend, err '%v'",
			unregisteredEntitiesWithWait.entities[i].Entity.Name, unregisteredEntitiesWithWait.entities[i].Err))
	}

	return composeEmitError(emitErrs, len(integrationData.DataSets))
}

func (e *emitter) RegisterEntities(entities []protocol.Entity) (registeredEntitiesNameToID, unregisteredEntityListWithWait) {
	// Bulk update them (after checking our datastore if they exist)
	// add entity ID to metric annotations
	return e.idProvider.ResolveEntities(entities)
}

func emitInventory(
	emitter agent.PluginEmitter,
	metadata integration.Definition,
	integrationMetadata protocol.IntegrationMetadata,
	entityID entity.ID,
	dataSet protocol.Dataset,
	labels map[string]string) {
	logEntry := elog.WithField("action", "EmitV4DataSet")

	integrationUser := metadata.ExecutorConfig.User

	if len(dataSet.Inventory) > 0 {
		inventoryDataSet := legacy.BuildInventoryDataSet(
			logEntry, dataSet.Inventory, labels, integrationUser, integrationMetadata.Name,
			dataSet.Entity.Name)
		entityKey := entity.Key(dataSet.Entity.Name)
		emitter.EmitInventory(inventoryDataSet, entity.New(entityKey, entityID))
	}
}

func emitEvent(
	emitter agent.PluginEmitter,
	metadata integration.Definition,
	dataSet protocol.Dataset,
	labels map[string]string) {

	integrationUser := metadata.ExecutorConfig.User
	for _, event := range dataSet.Events {
		normalizedEvent := legacy.
			NormalizeEvent(elog, event, labels, integrationUser, dataSet.Entity.Name)
		if normalizedEvent != nil {
			emitter.EmitEvent(normalizedEvent, entity.Key(dataSet.Entity.Name))
		}
	}
}

// Replace entity name by applying entity rewrites and replacing loopback
func replaceEntityName(entity protocol.Entity, entityRewrite []data.EntityRewrite, agentShortName string) {
	newName := legacy.ApplyEntityRewrite(entity.Name, entityRewrite)
	newName = http.ReplaceLocalhost(newName, agentShortName)
	entity.Name = newName
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

// Returns a composed error which describes all the errors found during the emit process of each data set
func composeEmitError(emitErrs []error, dataSetLenght int) error {
	if len(emitErrs) == 0 {
		return nil
	}

	composedError := fmt.Sprintf("%d out of %d datasets could not be emitted. Reasons: ", len(emitErrs), dataSetLenght)
	messages := map[string]struct{}{}

	for _, err := range emitErrs {
		msg := err.Error()
		if _, ok := messages[msg]; !ok { // avoid logging repeated error messages
			messages[msg] = struct{}{}
			composedError += msg + ","
		}
	}
	return errors.New(composedError[:len(composedError)-1])
}

func wrapError(err error, datasetLen int) error {
	composedError := fmt.Sprintf("%d out of %d datasets could not be emitted. Reasons: %v", datasetLen, datasetLen, err)
	return errors.New(composedError)
}
