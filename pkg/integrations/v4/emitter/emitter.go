// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package emitter

import (
	_context "context"
	"errors"
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/identity-client"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/legacy"
	_nethttp "net/http"

	"github.com/newrelic/infrastructure-agent/internal/feature_flags"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/dm"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

var (
	// Errors

	ProtocolV4NotEnabledErr = errors.New("integration protocol version 4 is not enabled")
	NoContentToParseErr     = errors.New("no content to parse")

	// internal

	elog = log.WithComponent("integrations.emitter.Legacy")
)

// Emitter submits the metrics to the next stage of the pipeline
type Emitter interface {
	Emit(metadata integration.Definition, ExtraLabels data.Map, entityRewrite []data.EntityRewrite, integrationJSON []byte) error
}

// IdentityClient registers entities
type IdentityClient interface {
	RegisterPost(
		ctx _context.Context,
		userAgent string,
		xLicenseKey string,
		registerRequest identity.RegisterRequest,
		localVarOptionals *identity.RegisterPostOpts) (identity.RegisterResponse, *_nethttp.Response, error)

	ConnectPost(
		ctx _context.Context,
		userAgent string,
		xLicenseKey string,
		connectRequest identity.ConnectRequest,
		localVarOptionals *identity.ConnectPostOpts) (identity.ConnectResponse, *_nethttp.Response, error)
}

func NewIntegrationEmitter(a *agent.Agent,
	dmSender dm.MetricsSender,
	identityClient IdentityClient,
	ffRetriever feature_flags.Retriever,
	license string,
	userAgent string) Emitter {
	return &Legacy{
		Context:             a.Context,
		MetricsSender:       dmSender,
		ForceProtocolV2ToV3: true,
		FFRetriever:         ffRetriever,
		identityClient:      identityClient,
		license:             license,
		userAgent:           userAgent,
	}
}

// Legacy abstracts all the complexities of the current Agent emitter for a better decoupling of the
// integrations package from the whole agent complexities
type Legacy struct {
	Context             agent.AgentContext
	MetricsSender       dm.MetricsSender
	ForceProtocolV2ToV3 bool
	FFRetriever         feature_flags.Retriever
	identityClient      IdentityClient
	userAgent           string
	license             string
}

func (e *Legacy) Emit(metadata integration.Definition, extraLabels data.Map, entityRewrite []data.EntityRewrite, integrationJSON []byte) error {
	protocolVersion, err := protocol.VersionFromPayload(integrationJSON, e.ForceProtocolV2ToV3)
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
		pluginDataV4, err := ParsePayloadV4(integrationJSON, e.FFRetriever)
		if err != nil {
			elog.WithError(err).WithField("output", string(integrationJSON)).Warn("can't parse v4 integration output")
			return err
		}

		return e.EmitV4(metadata, extraLabels, entityRewrite, pluginDataV4)
	}

	pluginDataV3, err := protocol.ParsePayload(integrationJSON, protocolVersion)
	if err != nil {
		elog.WithError(err).WithField("output", string(integrationJSON)).Warn("can't parse integration output")
		return err
	}

	return e.emitV3(metadata, extraLabels, entityRewrite, pluginDataV3, protocolVersion)
}

func (e *Legacy) emitV3(
	metadata integration.Definition,
	extraLabels data.Map,
	entityRewrite []data.EntityRewrite,
	pluginData protocol.PluginDataV3,
	protocolVersion int) error {
	var emitErrs []error

	pgId := metadata.PluginID(pluginData.Name)
	plugin := agent.NewExternalPluginCommon(pgId, e.Context, metadata.Name)

	labels, extraAnnotations := metadata.LabelsAndExtraAnnotations(extraLabels)

	for _, dataset := range pluginData.DataSets {
		err := legacy.EmitDataSet(
			e.Context,
			&plugin,
			pluginData.Name,
			pluginData.IntegrationVersion,
			metadata.ExecutorConfig.User,
			dataset,
			extraAnnotations,
			labels,
			entityRewrite,
			protocolVersion)
		if err != nil {
			emitErrs = append(emitErrs, err)
		}
	}

	return composeEmitError(emitErrs, len(pluginData.DataSets))
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
