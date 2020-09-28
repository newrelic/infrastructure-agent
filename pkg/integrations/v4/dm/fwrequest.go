// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package dm

import (
	"strings"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
)

// FwRequest stores integration telemetry data & metadata required from protocol v4 to be processed
// before it gets forwarded to NR telemetry SDK.
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

func (d FwRequest) PluginID() ids.PluginID {
	return d.Definition.PluginID(d.Data.Integration.Name)
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

	return labels, extraAnnotations
}
