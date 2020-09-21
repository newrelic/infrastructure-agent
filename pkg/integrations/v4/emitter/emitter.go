// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package emitter

import (
	"errors"
	"fmt"

	"github.com/newrelic/infrastructure-agent/pkg/integrations/legacy"

	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/handler"
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
	elog = log.WithComponent("integrations.emitter.Emittor")
)

// Emitter forwards agent/integration payload to  parser & processors (entity ID decoration...)
type Emitter interface {
	Emit(metadata integration.Definition, ExtraLabels data.Map, entityRewrite []data.EntityRewrite, integrationJSON []byte) error
}

type Agent interface {
	GetContext() agent.AgentContext
}

func NewIntegrationEmittor(
	a Agent,
	dmEmitter dm.Emitter,
	ffRetriever feature_flags.Retriever) Emitter {
	return &Emittor{
		aCtx:                a.GetContext(),
		forceProtocolV2ToV3: true,
		ffRetriever:         ffRetriever,
		dmEmitter:           dmEmitter,
	}
}

// Emittor actual Emitter for all integration protocol versions.
type Emittor struct {
	aCtx                agent.AgentContext
	forceProtocolV2ToV3 bool
	ffRetriever         feature_flags.Retriever
	dmEmitter           dm.Emitter
}

func (e *Emittor) Emit(metadata integration.Definition, extraLabels data.Map, entityRewrite []data.EntityRewrite, integrationJSON []byte) error {
	protocolVersion, err := protocol.VersionFromPayload(integrationJSON, e.forceProtocolV2ToV3)
	if err != nil {
		elog.
			WithError(err).
			WithField("protocol", protocolVersion).
			WithField("output", string(integrationJSON)).
			Warn("error retrieving integration protocol version")
		return err
	}

	// dimensional metrics
	if protocolVersion == protocol.V4 {
		pluginDataV4, err := dm.ParsePayloadV4(integrationJSON, e.ffRetriever)
		if err != nil {
			elog.WithError(err).WithField("output", string(integrationJSON)).Warn("can't parse v4 integration output")
			return err
		}

		dto := dm.NewDTO(metadata, extraLabels, entityRewrite, pluginDataV4)
		if enabled, exists := e.ffRetriever.GetFeatureFlag(handler.FlagDMRegisterEnable); exists && enabled {
			e.dmEmitter.Send(dto)
		} else {
			e.dmEmitter.SendWithoutRegister(dto)
		}
		return nil
	}

	pluginDataV3, err := protocol.ParsePayload(integrationJSON, protocolVersion)
	if err != nil {
		elog.WithError(err).WithField("output", string(integrationJSON)).Warn("can't parse integration output")
		return err
	}

	dto := integration.DTOV3{
		DTOMeta: integration.DTOMeta{
			Definition:    metadata,
			ExtraLabels:   extraLabels,
			EntityRewrite: entityRewrite,
		},
		Data: pluginDataV3,
	}
	return e.emitV3(dto, protocolVersion)
}

func (e *Emittor) emitV3(dto integration.DTOV3, protocolVersion int) error {
	plugin := agent.NewExternalPluginCommon(dto.Definition.PluginID(dto.Data.Name), e.aCtx, dto.Definition.Name)
	labels, extraAnnotations := dto.LabelsAndExtraAnnotations()

	var emitErrs []error
	for _, dataset := range dto.Data.DataSets {
		err := legacy.EmitDataSet(
			e.aCtx,
			&plugin,
			dto.Definition.Name,
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
