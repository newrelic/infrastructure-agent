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
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
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

// DTO stores integration protocol v4 received data and required metadata to be processed before
// submission.
type DTO struct {
	Metadata        integration.Definition
	ExtraLabels     data.Map
	EntityRewrite   []data.EntityRewrite
	IntegrationData protocol.DataV4
}

type Agent interface {
	GetContext() agent.AgentContext
}

type emitter struct {
	metricsSender MetricsSender
	agentContext  agent.AgentContext
	idProvider    idProviderInterface
}

type Emitter interface {
	Send(DTO)
	SendWithoutRegister(DTO)
}

func NewDTO(metadata integration.Definition,
	extraLabels data.Map,
	entityRewrite []data.EntityRewrite,
	integrationData protocol.DataV4) DTO {
	return DTO{
		Metadata:        metadata,
		ExtraLabels:     extraLabels,
		EntityRewrite:   entityRewrite,
		IntegrationData: integrationData,
	}
}

func (d DTO) PluginID() ids.PluginID {
	return d.Metadata.PluginID(d.IntegrationData.Integration.Name)
}

func NewEmitter(
	agentContext agent.AgentContext,
	dmSender MetricsSender,
	idProvider idProviderInterface) Emitter {

	return &emitter{
		agentContext:  agentContext,
		metricsSender: dmSender,
		idProvider:    idProvider,
	}
}

func (e *emitter) SendWithoutRegister(dto DTO) {
	metadata := dto.Metadata
	extraLabels := dto.ExtraLabels
	entityRewrite := dto.EntityRewrite
	integrationData := dto.IntegrationData

	var emitErrs []error

	plugin := agent.NewExternalPluginCommon(dto.PluginID(), e.agentContext, metadata.Name)

	labels, extraAnnotations := metadata.LabelsAndExtraAnnotations(extraLabels)

	var err error

	emitV4DataSet := func(
		idLookup agent.IDLookup,
		metricsSender MetricsSender,
		emitter agent.PluginEmitter,
		metadata integration.Definition,
		integrationMetadata protocol.IntegrationMetadata,
		dataSet protocol.Dataset,
		labels map[string]string,
		extraAnnotations map[string]string,
		entityRewrite []data.EntityRewrite) error {

		logEntry := elog.WithField("action", "EmitV4DataSet")

		replaceEntityNameWithoutRegister := func(entity protocol.Entity, entityRewrite []data.EntityRewrite, idLookup agent.IDLookup) error {
			// Replace entity name by applying entity rewrites and replacing loopback
			newName := legacy.ApplyEntityRewrite(entity.Name, entityRewrite)

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

		integrationUser := metadata.ExecutorConfig.User

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
			IntegrationInterval:         metadata.Interval,
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

	// TODO error handling
	elog.Error(composeEmitError(emitErrs, len(integrationData.DataSets)).Error())
}

func (e *emitter) Send(dto DTO) {
	agentShortName, err := e.agentContext.IDLookup().AgentShortEntityName()
	if err != nil {
		elog.
			WithError(err).
			WithField("integration", dto.Metadata.Name).
			Errorf("cannot determine agent short name")
		return
	}

	plugin := agent.NewExternalPluginCommon(dto.PluginID(), e.agentContext, dto.Metadata.Name)
	labels, extraAnnotations := dto.Metadata.LabelsAndExtraAnnotations(dto.ExtraLabels)

	var entities []protocol.Entity
	datasetsByEntityName := make(map[string]protocol.Dataset, len(dto.IntegrationData.DataSets))
	// Collect All entities
	for _, ds := range dto.IntegrationData.DataSets {
		entities = append(entities, ds.Entity)
		datasetsByEntityName[ds.Entity.Name] = ds
	}

	var emitErrs []error
	registeredEntities, unregisteredEntitiesWithWait := e.RegisterEntities(entities)

	for entityName, entityID := range registeredEntities {
		func(dataset protocol.Dataset, entityID entity.ID) {
			// for dataset.Entity call emitV4DataSet function with entity ID

			dataset.Common.Attributes[nrEntityId] = entityID.String()
			replaceEntityName(dataset.Entity, dto.EntityRewrite, agentShortName)

			emitInventory(
				&plugin,
				dto.Metadata,
				dto.IntegrationData.Integration,
				entityID,
				dataset,
				labels,
			)

			emitEvent(
				&plugin,
				dto.Metadata,
				dataset,
				labels,
			)

			dmProcessor := IntegrationProcessor{
				IntegrationInterval:         dto.Metadata.Interval,
				IntegrationLabels:           labels,
				IntegrationExtraAnnotations: extraAnnotations,
			}

			metrics := dmProcessor.ProcessMetrics(dataset.Metrics, dataset.Common, dataset.Entity)
			if err := e.metricsSender.SendMetricsWithCommonAttributes(dataset.Common, metrics); err != nil {
				// TODO error handling
			}
		}(datasetsByEntityName[entityName], entityID)
	}

	if len(unregisteredEntitiesWithWait.entities) == 0 {
		// TODO error handling
		return
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
		// TODO error handling
		elog.Error(composeEmitError(emitErrs, len(dto.IntegrationData.DataSets)).Error())
		return
	}

	// TODO error handling
	elog.Error(composeEmitError(emitErrs, len(dto.IntegrationData.DataSets)).Error())
	return
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
