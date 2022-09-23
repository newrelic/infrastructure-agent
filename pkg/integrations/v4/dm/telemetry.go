// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package dm

import (
	"errors"
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"io"
	"math"
	"net/http"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent/id"
	telemetry "github.com/newrelic/infrastructure-agent/pkg/backend/telemetryapi"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

const noCalculationMadeErrMsg = "no calculation made"

var errNoCalculation = errors.New(noCalculationMadeErrMsg)

var telemetryLogger = log.WithComponent("Telemetry")

type telemetryErrLogger struct {
	level string
}

func (l *telemetryErrLogger) Write(p []byte) (n int, err error) {
	switch l.level {
	case "error":
		telemetryLogger.Error(string(p))
	case "debug":
		telemetryLogger.Debug(string(p))
	case "audit":
		// payload should be logged only when customer enabled dm.submission feature traces.
		telemetryLogger.WithField(config.TracesFieldName, config.FeatureTrace)
	}

	return len(p), nil
}

func newTelemetryErrorLogger(level string) io.Writer {
	return &telemetryErrLogger{level: level}
}

func newTelemetryHarverster(conf MetricsSenderConfig, transport http.RoundTripper, idProvide id.Provide) (*telemetry.Harvester, error) {
	return telemetry.NewHarvester(
		telemetry.ConfigAPIKey(conf.LicenseKey),
		telemetry.ConfigBasicErrorLogger(newTelemetryErrorLogger("error")),
		telemetry.ConfigBasicDebugLogger(newTelemetryErrorLogger("debug")),
		telemetry.ConfigBasicAuditLogger(newTelemetryErrorLogger("audit")),
		telemetryHarvesterWithTransport(transport, conf.LicenseKey, idProvide),
		telemetryHarvesterWithMetricApiUrl(conf.MetricApiURL),
		telemetryHarvesterWithFedramp(conf.Fedramp),
		telemetry.ConfigHarvestPeriod(conf.SubmissionPeriod),
		telemetry.ConfigMaxEntitiesPerRequest(conf.MaxEntitiesPerReq),
		telemetry.ConfigMaxEntitiesPerBatch(conf.MaxEntitiesPerBatch),
	)
}

func telemetryHarvesterWithTransport(transport http.RoundTripper, licenseKey string, idProvide id.Provide) func(*telemetry.Config) {
	return func(config *telemetry.Config) {
		config.Client.Transport = newTransport(transport, licenseKey, idProvide)
	}
}

func telemetryHarvesterWithMetricApiUrl(metricApiUrl string) func(*telemetry.Config) {
	return func(config *telemetry.Config) {
		config.MetricsURLOverride = metricApiUrl
	}
}

func telemetryHarvesterWithFedramp(fedramp bool) func(*telemetry.Config) {
	return func(config *telemetry.Config) {
		config.Fedramp = fedramp
	}
}

type Conversion struct {
	toTelemetry         Converter
	toMultipleTelemetry DerivingConvertor
}

func (c *Conversion) convert(metric protocol.Metric) ([]telemetry.Metric, error) {
	var result []telemetry.Metric
	if c.toTelemetry != nil {
		converted, err := c.toTelemetry.from(metric)
		if err != nil {
			return nil, err
		}
		result = append(result, converted)
	}

	if c.toMultipleTelemetry != nil {
		derivedMetrics, err := c.toMultipleTelemetry.derivedFrom(metric)
		if err != nil {
			return nil, err
		}
		result = append(result, derivedMetrics...)
	}
	return result, nil
}

type Converter interface {
	from(metric protocol.Metric) (telemetry.Metric, error)
}

type DerivingConvertor interface {
	// derivedFrom is used when a metric is transformed into multiple telemetry metrics.
	derivedFrom(metric protocol.Metric) ([]telemetry.Metric, error)
}

type Count struct {
	calculate *Cumulative
}

func (c Count) from(metric protocol.Metric) (telemetry.Metric, error) {
	if c.shouldCalculate() {
		return c.calculate.from(metric)
	}

	value, err := metric.NumericValue()
	if err != nil {
		return nil, err
	}
	return telemetry.Count{
		Name:       metric.Name,
		Attributes: metric.Attributes,
		Value:      value,
		Timestamp:  metric.Time(),
		Interval:   metric.IntervalDuration(),
	}, nil
}

func (c *Count) shouldCalculate() bool {
	return c.calculate != nil
}

type Gauge struct {
	calculate *Rate
}

func (g Gauge) from(metric protocol.Metric) (telemetry.Metric, error) {
	if g.shouldCalculate() {
		return g.calculate.from(metric)
	}

	value, err := metric.NumericValue()
	if err != nil {
		return nil, err
	}
	return telemetry.Gauge{
		Name:       metric.Name,
		Attributes: metric.Attributes,
		Value:      value,
		Timestamp:  metric.Time(),
	}, nil
}

func (g *Gauge) shouldCalculate() bool {
	return g.calculate != nil
}

type Summary struct {
}

func (Summary) from(metric protocol.Metric) (telemetry.Metric, error) {
	value, err := metric.SummaryValue()

	if err != nil {
		return nil, err
	}

	return telemetry.Summary{
		Name:       metric.Name,
		Attributes: metric.Attributes,
		Count:      value.Count,
		Sum:        value.Sum,
		Min:        value.Min,
		Max:        value.Max,
		Timestamp:  metric.Time(),
		Interval:   metric.IntervalDuration(),
	}, nil
}

type PrometheusHistogram struct {
	calculate *Cumulative
}

func (ph PrometheusHistogram) derivedFrom(metric protocol.Metric) ([]telemetry.Metric, error) {
	var result []telemetry.Metric
	value, err := metric.GetPrometheusHistogramValue()

	if err != nil {
		return nil, err
	}

	if ph.calculate != nil {
		metricName := metric.Name + "_sum"
		sumCount, ok := ph.calculate.get(metricName, metric.Attributes, float64(*value.SampleCount), metric.Time())
		if ok {
			result = append(result, telemetry.Summary{
				Name:       metricName,
				Attributes: metric.Attributes,
				Count:      1,
				Sum:        sumCount.Value,
				Min:        math.NaN(),
				Max:        math.NaN(),
				Timestamp:  metric.Time(),
				Interval:   sumCount.Interval,
			})
		} else {
			telemetryLogger.WithField("name", metricName).WithField("metric-type", metric.Type).Debug(noCalculationMadeErrMsg)
		}

		metricName = metric.Name + "_bucket"
		for _, b := range value.Buckets {
			bucketAttrs := metric.CopyAttrs()
			bucketAttrs["le"] = fmt.Sprintf("%g", *b.UpperBound)

			bucketCount, ok := ph.calculate.get(
				metricName,
				bucketAttrs,
				float64(*b.CumulativeCount),
				metric.Time(),
			)
			if ok {
				result = append(result, bucketCount)
			} else {
				telemetryLogger.WithField("name", metricName).WithField("metric-type", metric.Type).Debug(noCalculationMadeErrMsg)
			}
		}
	}
	return result, nil
}

type PrometheusSummary struct {
	calculate *Cumulative
}

func (p PrometheusSummary) derivedFrom(metric protocol.Metric) ([]telemetry.Metric, error) {
	var result []telemetry.Metric
	value, err := metric.GetPrometheusSummaryValue()

	if err != nil {
		return nil, err
	}

	if p.calculate != nil {
		metricName := metric.Name + "_sum"
		sumMetric, ok := p.calculate.get(metricName, metric.Attributes, value.SampleSum, metric.Time())

		if ok {
			result = append(result, telemetry.Summary{
				Name:       metricName,
				Attributes: metric.Attributes,
				Count:      1,
				Sum:        sumMetric.Value,
				Min:        math.NaN(),
				Max:        math.NaN(),
				Timestamp:  metric.Time(),
				Interval:   sumMetric.Interval,
			})
		} else {
			telemetryLogger.WithField("name", metricName).WithField("metric-type", metric.Type).Debug(noCalculationMadeErrMsg)
		}

		metricName = metric.Name + "_count"
		countMetric, ok := p.calculate.get(metricName, metric.Attributes, value.SampleCount, metric.Time())

		if ok {
			result = append(result, countMetric)
		} else {
			telemetryLogger.WithField("name", metricName).WithField("metric-type", metric.Type).Debug(noCalculationMadeErrMsg)
		}

		for _, q := range value.Quantiles {
			quantileAttrs := metric.CopyAttrs()
			quantileAttrs["quantile"] = fmt.Sprintf("%g", q.Quantile)
			result = append(result, telemetry.Gauge{
				Name:       metric.Name,
				Attributes: quantileAttrs,
				Value:      q.Value,
				Timestamp:  metric.Time(),
			})
		}
	}
	return result, nil
}

type Rate struct {
	get func(name string,
		attributes map[string]interface{},
		val float64,
		now time.Time) (gauge telemetry.Gauge, valid bool)
}

func (r Rate) from(metric protocol.Metric) (telemetry.Metric, error) {
	value, err := metric.NumericValue()

	if err != nil {
		return nil, err
	}

	m, ok := r.get(metric.Name, metric.Attributes, value, metric.Time())

	if !ok {
		telemetryLogger.WithField("name", metric.Name).WithField("metric-type", metric.Type).Debug(noCalculationMadeErrMsg)
		return nil, errNoCalculation
	}

	return m, nil
}

type Cumulative struct {
	get func(name string,
		attributes map[string]interface{},
		val float64,
		now time.Time) (count telemetry.Count, valid bool)
}

func (c Cumulative) from(metric protocol.Metric) (telemetry.Metric, error) {
	value, err := metric.NumericValue()

	if err != nil {
		return nil, err
	}

	m, ok := c.get(metric.Name, metric.Attributes, value, metric.Time())

	if !ok {
		telemetryLogger.WithField("name", metric.Name).WithField("metric-type", metric.Type).Debug(noCalculationMadeErrMsg)
		return nil, errNoCalculation
	}

	return m, nil
}
