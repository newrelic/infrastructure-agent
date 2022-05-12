// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package dm

import (
	"net/http"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent/id"

	telemetry "github.com/newrelic/infrastructure-agent/pkg/backend/telemetryapi"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/dm/cumulative"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/dm/rate"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

var logger = log.WithComponent("DimensionalMetricsSender")

type MetricsSender interface {
	SendMetrics(metrics []protocol.Metric)
	SendMetricsWithCommonAttributes(commonAttributes protocol.Common, metrics []protocol.Metric) error
}

type MetricsSenderConfig struct {
	Fedramp             bool
	LicenseKey          string
	MetricApiURL        string
	SubmissionPeriod    time.Duration
	MaxEntitiesPerReq   int
	MaxEntitiesPerBatch int
}

func NewConfig(url string, fedramp bool, licenseKey string, submissionPeriod time.Duration, maxEntitiesPerReq int, maxEntitiesPerBatch int) MetricsSenderConfig {
	return MetricsSenderConfig{
		Fedramp:             fedramp,
		LicenseKey:          licenseKey,
		MetricApiURL:        url,
		SubmissionPeriod:    submissionPeriod,
		MaxEntitiesPerReq:   maxEntitiesPerReq,
		MaxEntitiesPerBatch: maxEntitiesPerBatch,
	}
}

// NewDMSender creates a Dimensional Metrics sender.
func NewDMSender(config MetricsSenderConfig, transport http.RoundTripper, idProvide id.Provide) (s MetricsSender, err error) {
	s = &sender{
		harvester: NewLazyLoadedHarvester(config, transport, idProvide),
		calculator: Calculator{
			rate:  rate.NewCalculator(),
			delta: cumulative.NewDeltaCalculator(),
		},
	}
	return
}

type sender struct {
	harvester  metricHarvester
	calculator Calculator
}

type Calculator struct {
	delta deltaCalculator
	rate  rate.Calculator
}

type deltaCalculator interface {
	//GetCumulativeCount creates a count metric from the difference between the values and
	//timestamps of multiple calls.  If this is the first time the name/attributes
	//combination has been seen then the `valid` return value will be false.
	CountMetric(
		name string,
		attributes map[string]interface{},
		val float64,
		now time.Time) (count telemetry.Count, valid bool)
}

type metricHarvester interface {
	RecordMetric(m telemetry.Metric)
	RecordInfraMetrics(commonAttribute telemetry.Attributes, metrics []telemetry.Metric) error
}

// Deprecated: Use SendMetricsWithCommonAttributes
func (s *sender) SendMetrics(metrics []protocol.Metric) {
	for _, metric := range metrics {

		var c Conversion

		switch metric.Type {
		case "gauge":
			c = Conversion{toTelemetry: Gauge{}}
		case "count":
			c = Conversion{toTelemetry: Count{}}
		case "summary":
			c = Conversion{toTelemetry: Summary{}}
		case "rate":
			c = Conversion{toTelemetry: Gauge{calculate: &Rate{get: s.calculator.rate.GetRate}}}
		case "cumulative-rate":
			c = Conversion{toTelemetry: Gauge{calculate: &Rate{get: s.calculator.rate.GetCumulativeRate}}}
		case "cumulative-count":
			c = Conversion{toTelemetry: Count{calculate: &Cumulative{get: s.calculator.delta.CountMetric}}}
		case "prometheus-summary":
			c = Conversion{toMultipleTelemetry: PrometheusSummary{calculate: &Cumulative{get: s.calculator.delta.CountMetric}}}
		case "prometheus-histogram":
			c = Conversion{toMultipleTelemetry: PrometheusHistogram{calculate: &Cumulative{get: s.calculator.delta.CountMetric}}}
		default:
			logger.WithField("name", metric.Name).WithField("metric-type", metric.Name).Warn("received an unknown metric type")
			continue
		}

		recMetric, err := c.convert(metric)

		if err != nil {
			if err != errNoCalculation {
				logger.WithField("name", metric.Name).WithField("metric-type", metric.Type).WithError(err).Error("received a metric with invalid value")
			}
			continue
		}

		for _, m := range recMetric {
			s.harvester.RecordMetric(m)
		}
	}
}

func (s *sender) SendMetricsWithCommonAttributes(commonAttributes protocol.Common, metrics []protocol.Metric) error {
	dMetrics := s.convertMetrics(metrics)
	if len(dMetrics) > 0 {
		return s.harvester.RecordInfraMetrics(commonAttributes.Attributes, dMetrics)
	}
	return nil
}

func (s *sender) convertMetrics(metrics []protocol.Metric) []telemetry.Metric {
	var dMetrics []telemetry.Metric

	for _, metric := range metrics {

		var c Conversion

		switch metric.Type {
		case "gauge":
			c = Conversion{toTelemetry: Gauge{}}
		case "count":
			c = Conversion{toTelemetry: Count{}}
		case "summary":
			c = Conversion{toTelemetry: Summary{}}
		case "rate":
			c = Conversion{toTelemetry: Gauge{calculate: &Rate{get: s.calculator.rate.GetRate}}}
		case "cumulative-rate":
			c = Conversion{toTelemetry: Gauge{calculate: &Rate{get: s.calculator.rate.GetCumulativeRate}}}
		case "cumulative-count":
			c = Conversion{toTelemetry: Count{calculate: &Cumulative{get: s.calculator.delta.CountMetric}}}
		case "prometheus-summary":
			c = Conversion{toMultipleTelemetry: PrometheusSummary{calculate: &Cumulative{get: s.calculator.delta.CountMetric}}}
		case "prometheus-histogram":
			c = Conversion{toMultipleTelemetry: PrometheusHistogram{calculate: &Cumulative{get: s.calculator.delta.CountMetric}}}
		default:
			logger.WithField("name", metric.Name).WithField("metric-type", metric.Name).Warn("received an unknown metric type")
			continue
		}

		recMetrics, err := c.convert(metric)

		if err != nil {
			if err != errNoCalculation {
				// TODO: Return error or not?
				logger.WithField("name", metric.Name).WithField("metric-type", metric.Type).WithError(err).Error("received a metric with invalid value")
			}
			continue
		}
		dMetrics = append(dMetrics, recMetrics...)
	}
	return dMetrics
}
