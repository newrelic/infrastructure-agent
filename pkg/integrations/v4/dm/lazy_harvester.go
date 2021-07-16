// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package dm

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/newrelic/infrastructure-agent/internal/agent/id"
	telemetry "github.com/newrelic/infrastructure-agent/pkg/backend/telemetryapi"
)

func NewLazyLoadedHarvester(config MetricsSenderConfig, transport http.RoundTripper, idProvide id.Provide) metricHarvester {
	return &lazyLoadHarvester{
		config:    config,
		transport: transport,
		idProvide: idProvide,
	}
}

type lazyLoadHarvester struct {
	harvester metricHarvester
	once      sync.Once
	config    MetricsSenderConfig
	transport http.RoundTripper
	idProvide id.Provide
}

func (l *lazyLoadHarvester) load() (err error) {
	l.once.Do(func() {
		l.harvester, err = newTelemetryHarverster(l.config, l.transport, l.idProvide)
	})
	return
}

func (l *lazyLoadHarvester) RecordMetric(m telemetry.Metric) {
	if l.harvester == nil {
		if err := l.load(); err != nil {
			logger.WithError(err).Error("cannot load telemetry harvester, dimensional metrics will be lost")
			return
		}
	}

	l.harvester.RecordMetric(m)
}

func (l *lazyLoadHarvester) RecordInfraMetrics(commonAttribute telemetry.Attributes, metrics []telemetry.Metric) error {
	if l.harvester == nil {
		if err := l.load(); err != nil {
			return fmt.Errorf("cannot load telemetry harvester: %v", err)
		}
	}

	return l.harvester.RecordInfraMetrics(commonAttribute, metrics)
}
