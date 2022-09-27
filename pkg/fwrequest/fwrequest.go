// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package fwrequest

import (
	"strconv"
	"strings"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
)

const (
	EntityIdAttribute                = "nr.entity.id"
	InstrumentationVersionAttribute  = "instrumentation.version"
	InstrumentationNameAttribute     = "instrumentation.name"
	InstrumentationProviderAttribute = "instrumentation.provider"
	CollectorNameAttribute           = "collector.name"
	CollectorVersionAttribute        = "collector.version"
	labelPrefix                      = "label."
	tagsPrefix                       = "tags."
	labelPrefixTrim                  = 6
	newRelicProvider                 = "newRelic"
	agentCollector                   = "infrastructure-agent"
)

// EntityFwRequest stores an integration single entity payload to be processed before it gets
// forwarded to NR telemetry SDK.
type EntityFwRequest struct {
	FwRequestMeta
	Integration protocol.IntegrationMetadata
	Data        protocol.Dataset
}

func (r *EntityFwRequest) RegisteredWith(id entity.ID) {
	// attributes ID decoration
	if r.Data.Common.Attributes == nil {
		r.Data.Common.Attributes = make(map[string]interface{})
	}
	r.Data.Common.Attributes[EntityIdAttribute] = id.String()
}

func (r *EntityFwRequest) populateCommonAttributes(intMeta protocol.IntegrationMetadata, agentVersion string) {
	if r.Data.Common.Attributes == nil {
		r.Data.Common.Attributes = make(map[string]interface{})
	}
	r.Data.Common.Attributes[InstrumentationVersionAttribute] = intMeta.Version
	r.Data.Common.Attributes[InstrumentationNameAttribute] = intMeta.Name
	r.Data.Common.Attributes[InstrumentationProviderAttribute] = newRelicProvider
	r.Data.Common.Attributes[CollectorNameAttribute] = agentCollector
	r.Data.Common.Attributes[CollectorVersionAttribute] = agentVersion
}

func (r *EntityFwRequest) ID() entity.ID {
	// TODO candidate for optimization
	if r.Data.Common.Attributes != nil {
		if id, ok := r.Data.Common.Attributes[EntityIdAttribute]; ok {
			if idStr, ok := id.(string); ok {
				if idInt, err := strconv.Atoi(idStr); err == nil {
					return entity.ID(idInt)
				}
			}
		}
	}

	return entity.EmptyID
}

// FwRequest stores an integration payload with telemetry data & metadata required from protocol v4
// to be processed before it gets forwarded to NR telemetry SDK.
type FwRequest struct {
	FwRequestMeta
	Data protocol.DataV4
}

// FwRequestLegacy stores integration telemetry data & metadata required from protocol v3 to be
// processed before it gets forwarded to NR telemetry SDK.
type FwRequestLegacy struct {
	FwRequestMeta
	Data protocol.PluginDataV3
}

// FwRequestMeta stores integration required metadata for telemetry data to be processed before it
// gets forwarded to NR telemetry SDK.
type FwRequestMeta struct {
	Definition    integration.Definition
	ExtraLabels   data.Map
	EntityRewrite []data.EntityRewrite
}

func NewFwRequest(definition integration.Definition,
	extraLabels data.Map,
	entityRewrite []data.EntityRewrite,
	integrationData protocol.DataV4) FwRequest {
	return FwRequest{
		FwRequestMeta: FwRequestMeta{
			Definition:    definition,
			ExtraLabels:   extraLabels,
			EntityRewrite: entityRewrite,
		},
		Data: integrationData,
	}
}

func (r *FwRequest) PluginID() ids.PluginID {
	return r.Definition.PluginID(r.Data.Integration.Name)
}

func NewEntityFwRequest(
	entityDataSet protocol.Dataset,
	id entity.ID,
	reqMeta FwRequestMeta,
	intMeta protocol.IntegrationMetadata,
	agentVersion string,
) EntityFwRequest {
	r := EntityFwRequest{
		FwRequestMeta: reqMeta,
		Integration:   intMeta,
		Data:          entityDataSet,
	}
	if !id.IsEmpty() {
		r.RegisteredWith(id)
	}
	r.populateCommonAttributes(intMeta, agentVersion)

	return r
}

func NewFwRequestLegacy(definition integration.Definition,
	extraLabels data.Map,
	entityRewrite []data.EntityRewrite,
	integrationData protocol.PluginDataV3,
) FwRequestLegacy {

	return FwRequestLegacy{
		FwRequestMeta: FwRequestMeta{
			Definition:    definition,
			ExtraLabels:   extraLabels,
			EntityRewrite: entityRewrite,
		},
		Data: integrationData,
	}
}

func (d *FwRequestMeta) LabelsAndExtraAnnotations() (map[string]string, map[string]string) {
	labels := make(map[string]string, len(d.Definition.Labels)+len(d.ExtraLabels))
	extraAnnotations := make(map[string]string, len(d.ExtraLabels))

	for k, v := range d.Definition.Labels {
		labels[k] = v
	}

	for k, v := range d.ExtraLabels {
		if strings.HasPrefix(k, labelPrefix) {
			labels[k[labelPrefixTrim:]] = v
		} else {
			extraAnnotations[k] = v
		}
	}

	// Adding tags.
	for k, v := range d.Definition.Tags {
		extraAnnotations[tagsPrefix+k] = v
	}

	return labels, extraAnnotations
}
