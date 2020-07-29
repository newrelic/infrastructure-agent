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

type emitter struct {
	ffRetriever   feature_flags.Retriever
	metricsSender MetricsSender
	agentContext  agent.AgentContext
}

type Emitter interface {
	Send(
		metadata integration.Definition,
		extraLabels data.Map,
		entityRewrite []data.EntityRewrite,
		integrationJSON []byte) error
}

func NewEmitter(
	a *agent.Agent,
	dmSender MetricsSender,
	ffRetriever feature_flags.Retriever) Emitter {

	return &emitter{
		agentContext:  a.GetContext(),
		metricsSender: dmSender,
		ffRetriever:   ffRetriever,
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
	integrationData protocol.DataV4) error {
	var emitErrs []error

	pluginId := metadata.PluginID(integrationData.Integration.Name)
	plugin := agent.NewExternalPluginCommon(pluginId, e.agentContext, metadata.Name)

	labels, extraAnnotations := metadata.LabelsAndExtraAnnotations(extraLabels)

	var err error
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

	return composeEmitError(emitErrs, len(integrationData.DataSets))
}

func emitV4DataSet(
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
		normalizedEvent := legacy.
			NormalizeEvent(elog, event, labels, integrationUser, dataSet.Entity.Name)
		if normalizedEvent != nil {
			emitter.EmitEvent(normalizedEvent, entity.Key(dataSet.Entity.Name))
		}
	}

	dmProcessor := IntegrationProcessor{
		IntegrationInterval:         metadata.Interval,
		IntegrationLabels:           labels,
		IntegrationExtraAnnotations: extraAnnotations,
	}

	// TODO: register entities
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
			composedError += msg
		}
	}

	return errors.New(composedError)
}
