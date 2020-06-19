// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package dm

import (
	"fmt"
	"net/http"
	"time"

	"github.com/newrelic-forks/newrelic-telemetry-sdk-go/cumulative"
	"github.com/newrelic-forks/newrelic-telemetry-sdk-go/rate"
	"github.com/newrelic-forks/newrelic-telemetry-sdk-go/telemetry"

	"github.com/newrelic/infrastructure-agent/internal/agent/id"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/license"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

// endpoints
const (
	usDomain      = "metric-api.newrelic.com"
	euDomain      = "metric-api.newrelic.com"
	stagingDomain = "staging-metric-api.newrelic.com"
)

var logger = log.WithComponent("DimensionalMetricsSender")

type MetricsSender interface {
	SendMetrics(metrics []protocol.Metric)
}

type MetricsSenderConfig struct {
	LicenseKey       string
	MetricApiURL     string
	SubmissionPeriod time.Duration
}

func NewConfig(staging bool, licenseKey string, submissionPeriod time.Duration) MetricsSenderConfig {
	domain := usDomain
	if staging {
		domain = stagingDomain
	} else if license.IsRegionEU(licenseKey) {
		domain = euDomain
	}

	return MetricsSenderConfig{
		LicenseKey:       licenseKey,
		MetricApiURL:     fmt.Sprintf("https://%s/metric/v1/infra", domain),
		SubmissionPeriod: submissionPeriod,
	}
}

// NewDMSender creates a Dimensional Metrics sender.
func NewDMSender(config MetricsSenderConfig, transport http.RoundTripper, idContext *id.Context) (s MetricsSender, err error) {
	harvester, err := newTelemetryHarverster(config, transport, idContext.AgentIdentity)
	s = &sender{
		harvester: harvester,
		calculator: Calculator{
			rate:  rate.NewRateCalculator(),
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
	rate  rateCalculator
}

type rateCalculator interface {
	//GetRate creates a Gauge metric with rate of change based on the previous timestamp.
	//If no previous timestamp is found, returns false (as no calculation is made)
	//If a previous timestamp is found use it to get the elapsed time (in seconds) and use that as the denominator
	//Rate = value / (now - before)[s]
	GetRate(
		name string,
		attributes map[string]interface{},
		val float64,
		now time.Time) (gauge telemetry.Gauge, valid bool)

	//GetCumulativeRate creates a Gauge metric with rate of change based on the previous timestamp and value.
	//If no previous timestamp is found, returns false (as no calculation is made)
	//If a previous timestamp is found use it to get the elapsed time (in seconds) and use that as the denominator
	//Rate = value / (now - before)[s]
	GetCumulativeRate(
		name string,
		attributes map[string]interface{},
		val float64,
		now time.Time) (gauge telemetry.Gauge, valid bool)
}

type deltaCalculator interface {
	//GetCumulativeCount creates a count metric from the difference between the values and
	//timestamps of multiple calls.  If this is the first time the name/attributes
	//combination has been seen then the `valid` return value will be false.
	GetCumulativeCount(
		name string,
		attributes map[string]interface{},
		val float64,
		now time.Time) (count telemetry.Count, valid bool)
}

type metricHarvester interface {
	RecordMetric(m telemetry.Metric)
}

func (s *sender) SendMetrics(metrics []protocol.Metric) {
	for _, metric := range metrics {

		var c Conversion

		switch metric.Type {
		case "gauge":
			c = Conversion{Gauge{}}
		case "count":
			c = Conversion{Count{}}
		case "summary":
			c = Conversion{Summary{}}
		case "rate":
			c = Conversion{Gauge{calculate: &Rate{get: s.calculator.rate.GetRate}}}
		case "cumulative-rate":
			c = Conversion{Gauge{calculate: &Rate{get: s.calculator.rate.GetCumulativeRate}}}
		case "cumulative-count":
			c = Conversion{Count{calculate: &Cumulative{get: s.calculator.delta.GetCumulativeCount}}}
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

		s.harvester.RecordMetric(recMetric)
	}
}
