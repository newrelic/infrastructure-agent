// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package emitter

import (
	"errors"
	"fmt"

	"github.com/newrelic/infrastructure-agent/pkg/helpers"

	"github.com/newrelic/infrastructure-agent/pkg/fwrequest"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/legacy"
	"github.com/sirupsen/logrus"

	"github.com/newrelic/infrastructure-agent/internal/feature_flags"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/dm"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

var (
	// internal
	elog = log.WithComponent("integrations.emitter.Emitter")
)

// Emitter forwards agent/integration payload to  parser & processors (entity ID decoration...)
type Emitter interface {
	Emit(definition integration.Definition, ExtraLabels data.Map, entityRewrite []data.EntityRewrite, integrationJSON []byte) error
}

type Agent interface {
	GetContext() agent.AgentContext
}

func NewIntegrationEmittor(
	a Agent,
	dmEmitter dm.Emitter,
	ffRetriever feature_flags.Retriever) Emitter {
	return &VersionAwareEmitter{
		aCtx:                a.GetContext(),
		forceProtocolV2ToV3: true,
		ffRetriever:         ffRetriever,
		dmEmitter:           dmEmitter,
	}
}

// VersionAwareEmitter actual Emitter for all integration protocol versions.
type VersionAwareEmitter struct {
	aCtx                agent.AgentContext
	forceProtocolV2ToV3 bool
	ffRetriever         feature_flags.Retriever
	dmEmitter           dm.Emitter
}

func (e *VersionAwareEmitter) Emit(definition integration.Definition, extraLabels data.Map, entityRewrite []data.EntityRewrite, integrationJSON []byte) error {
	fields := logrus.Fields{
		"integration_name": definition.Name,
	}
	if definition.CfgProtocol != nil {
		fields["cfg_protocol_name"] = definition.CfgProtocol.ConfigName
		fields["parent_integration_name"] = definition.CfgProtocol.ParentName
	}

	envVarsForLogEntry := helpers.ObfuscateSensitiveDataFromMap(definition.ExecutorConfig.Environment)
	elog.WithTraceField("payload", string(integrationJSON)).WithField("env", envVarsForLogEntry).WithFields(fields).Debug("Received payload.")

	protocolVersion, err := protocol.VersionFromPayload(integrationJSON, e.forceProtocolV2ToV3)
	if err != nil {
		elog.WithError(err).WithFields(fields).Warn("error retrieving integration protocol version")
		return err
	}

	// Agent creating the Host entity (and decorating it correctly in the backend) in secure forward with Custom Attributes: pkg/plugins/plugins_linux.go:46
	// But in forward only there is no host entity and custom attributes are not being decorated.
	// Here then we add CustomAttributes to extraLabels in case we are in that mode.
	if e.aCtx.Config().IsForwardOnly {
		extraLabelsCopy := make(map[string]string)
		customAttributes := e.aCtx.Config().CustomAttributes.DataMap()

		for k, v := range extraLabels {
			extraLabelsCopy[k] = v
		}
		for k, v := range customAttributes {
			extraLabelsCopy[k] = v
		}

		extraLabels = extraLabelsCopy
	}

	// dimensional metrics
	if protocolVersion == protocol.V4 {
		pluginDataV4, err := dm.ParsePayloadV4(integrationJSON, e.ffRetriever)
		if err != nil {
			elog.WithError(err).WithFields(fields).Warn("can't parse v4 integration output")
			return err
		}
		agentResolver := e.aCtx.HostnameResolver()
		_, overrideHostname, _ := agentResolver.Query()
		// var dataSet protocol.Dataset

		for _, dataSet := range pluginDataV4.DataSets {
			// Only update hostname for windows services
			if overrideHostname != "" && dataSet.Entity.Type == "WIN_SERVICE" {
				dataSet.Entity.Metadata["hostname"] = overrideHostname
			}
		}
		e.dmEmitter.Send(fwrequest.NewFwRequest(definition, extraLabels, entityRewrite, pluginDataV4))
		return nil
	}

	pluginDataV3, err := protocol.ParsePayload(integrationJSON, protocolVersion)
	if err != nil {
		elog.WithError(err).WithFields(fields).Warn("can't parse integration output")
		return err
	}

	return e.emitV3(fwrequest.NewFwRequestLegacy(definition, extraLabels, entityRewrite, pluginDataV3), protocolVersion)
}

func (e *VersionAwareEmitter) emitV3(dto fwrequest.FwRequestLegacy, protocolVersion int) error {
	plugin := agent.NewExternalPluginCommon(dto.Definition.PluginID(dto.Data.Name), e.aCtx, dto.Definition.Name)
	labels, extraAnnotations := dto.LabelsAndExtraAnnotations()

	var emitErrs []error
	for _, dataset := range dto.Data.DataSets {
		err := legacy.EmitDataSet(
			e.aCtx,
			&plugin,
			dto.Data.Name,
			dto.Data.IntegrationVersion,
			dto.Definition.ExecutorConfig.User,
			dataset,
			extraAnnotations,
			labels,
			dto.EntityRewrite,
			protocolVersion)
		if err != nil {
			emitErrs = append(emitErrs, err)
		}
	}

	return composeEmitError(emitErrs, len(dto.Data.DataSets))
}

// Returns a composed error which describes all the errors found during the emit process of each data set
func composeEmitError(emitErrs []error, dataSetLength int) error {
	if len(emitErrs) == 0 {
		return nil
	}

	composedError := fmt.Sprintf("%d out of %d datasets could not be emitted. Reasons: ", len(emitErrs), dataSetLength)
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
