// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"context"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/api/metric"
	oprometheus "go.opentelemetry.io/otel/exporters/metric/prometheus"
	"go.opentelemetry.io/otel/label"
)

type opentelemetry struct {
	handler  *oprometheus.Exporter
	meter    *metric.Meter
	counters map[MetricName]metric.Int64Counter
}

func (o opentelemetry) GetHandler() http.Handler {
	return o.handler
}

func (o opentelemetry) Measure(metricType MetricType, name MetricName, val int64) {
	o.meter.RecordBatch(
		context.Background(),
		[]label.KeyValue{},
		o.counters[name].Measurement(val))
}

func (o opentelemetry) GetHttpTransport(base http.RoundTripper) http.RoundTripper {
	return otelhttp.NewTransport(base,
		otelhttp.WithMeterProvider(o.handler.MeterProvider()),
		otelhttp.WithMessageEvents(
			otelhttp.ReadEvents,
			otelhttp.WriteEvents))
}

func NewOpentelemetryExporter() (exporter Exporter, err error) {
	registry := prometheus.NewRegistry()
	registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	registry.MustRegister(prometheus.NewGoCollector())
	prometheusExporter, err := oprometheus.InstallNewPipeline(oprometheus.Config{
		Registry: registry,
	})
	if err != nil {
		return nil, err
	}
	meter := prometheusExporter.MeterProvider().Meter("newrelic.infra")

	counters := make(map[MetricName]metric.Int64Counter, 2)

	for metricName, metricRegistrationName := range metricsToRegister {
		counters[metricName] = metric.Must(meter).NewInt64Counter("newrelic.infra/instrumentation." + metricRegistrationName)
	}

	return &opentelemetry{
		handler:  prometheusExporter,
		counters: counters,
		meter:    &meter,
	}, err
}
