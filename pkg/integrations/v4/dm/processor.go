// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package dm

import (
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
	"time"
)

const (
	// These two constants can be found in old integrations as well
	labelPrefix     = "label."
	labelPrefixTrim = 6
)

type IntegrationProcessor struct {
	IntegrationInterval         time.Duration
	IntegrationLabels           map[string]string
	IntegrationExtraAnnotations map[string]string
}

// ProcessMetrics metrics processing (decoration)
func (p *IntegrationProcessor) ProcessMetrics(
	metrics []protocol.Metric,
	common protocol.Common,
	entity protocol.Entity) []protocol.Metric {
	now := time.Now().Unix()

	for i := range metrics {
		p.addTimestamp(&metrics[i], common.Timestamp, &now)
		p.addInterval(&metrics[i], common.Interval)
		p.addAttributes(&metrics[i], common.Attributes)
		p.addLabels(&metrics[i])
		p.addExtraAnnotations(&metrics[i])
		p.addAttributes(&metrics[i], entity.Metadata)
	}

	return metrics
}

// If metric doesn't have its own timestamp, add timestamp from common block (of present)
// or now
func (p *IntegrationProcessor) addTimestamp(
	metric *protocol.Metric, commonTimestamp *int64, now *int64) {
	if metric.Timestamp == nil {
		if commonTimestamp != nil {
			metric.Timestamp = commonTimestamp
		} else {
			metric.Timestamp = now
		}
	}
}

// it potentially adds interval to metric from common block or integration metadata (in this order
// of precedence) when count or summary don't provide specific value.
func (p *IntegrationProcessor) addInterval(m *protocol.Metric, commonBlockInterval *int64) {
	if m.Interval != nil || !m.Type.HasInterval() {
		return
	}

	if commonBlockInterval != nil {
		m.Interval = commonBlockInterval
	} else {
		i := int64(p.IntegrationInterval * time.Millisecond)
		m.Interval = &i
	}
}

// Add attributes to a metric. If a key is already defined at the metric level,
// it won't be overridden
func (p *IntegrationProcessor) addAttributes(
	metric *protocol.Metric, attributes map[string]interface{}) {
	if metric.Attributes == nil {
		metric.Attributes = make(map[string]interface{})
	}
	for k, v := range attributes {
		if _, ok := metric.Attributes[k]; !ok {
			metric.Attributes[k] = v
		}
	}
}

// Add integration labels to a metric, prefixing the key with label.
func (p *IntegrationProcessor) addLabels(metric *protocol.Metric) {
	for k, v := range p.IntegrationLabels {
		metric.Attributes[labelPrefix+k] = v
	}
}

// Add integration extra attributes to a metric. If a key is already defined
// at the metric level, it won't be overridden
func (p *IntegrationProcessor) addExtraAnnotations(metric *protocol.Metric) {
	for k, v := range p.IntegrationExtraAnnotations {
		if _, ok := metric.Attributes[k]; !ok {
			metric.Attributes[k] = v
		}
	}
}
