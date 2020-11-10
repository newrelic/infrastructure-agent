// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build go1.13

package instrumentation

import (
	"context"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/api/metric"
	oprometheus "go.opentelemetry.io/otel/exporters/metric/prometheus"
	"go.opentelemetry.io/otel/label"
)

type opentelemetry struct {
	handler http.Handler
	counter *metric.Int64Counter
	meter   *metric.Meter
}

func (o opentelemetry) GetHandler() http.Handler {
	return o.handler
}

func (o opentelemetry) IncrementSomething(val int64) {
	o.meter.RecordBatch(
		context.Background(),
		[]label.KeyValue{},
		o.counter.Measurement(val))
}

func NewOpentelemetryServer() (exporter Exporter, err error) {
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
	counter := metric.Must(meter).NewInt64Counter("newrelic.infra/instrumentation.counter")
	return &opentelemetry{
		handler: prometheusExporter,
		counter: &counter,
		meter:   &meter,
	}, err
}
