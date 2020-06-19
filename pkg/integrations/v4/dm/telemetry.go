// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package dm

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/newrelic-forks/newrelic-telemetry-sdk-go/telemetry"
	"github.com/newrelic/infrastructure-agent/internal/agent/id"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/trace"
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
		trace.Telemetry(string(p))
	}

	return len(p), nil
}

func newTelemetryErrorLogger(level string) io.Writer {
	return &telemetryErrLogger{level: level}
}

func newTelemetryHarverster(conf MetricsSenderConfig, transport http.RoundTripper, idnProvide id.Provide) (*telemetry.Harvester, error) {
	return telemetry.NewHarvester(
		telemetry.ConfigAPIKey(conf.LicenseKey),
		telemetry.ConfigBasicErrorLogger(newTelemetryErrorLogger("error")),
		telemetry.ConfigBasicDebugLogger(newTelemetryErrorLogger("debug")),
		telemetry.ConfigBasicAuditLogger(newTelemetryErrorLogger("audit")),
		telemetryHarvesterWithTransport(transport, conf.LicenseKey, idnProvide),
		telemetryHarvesterWithMetricApiUrl(conf.MetricApiURL),
		telemetry.ConfigHarvestPeriod(conf.SubmissionPeriod),
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

type Conversion struct {
	toTelemetry Converter
}

func (c *Conversion) convert(metric protocol.Metric) (telemetry.Metric, error) {
	return c.toTelemetry.from(metric)
}

type Converter interface {
	from(metric protocol.Metric) (telemetry.Metric, error)
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

type Summary struct{}

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
