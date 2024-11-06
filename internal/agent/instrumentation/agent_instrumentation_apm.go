// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/newrelic/newrelic-telemetry-sdk-go/telemetry"
	"github.com/sirupsen/logrus"

	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/hostname"
)

const transactionInContextKey = iota

const (
	appName                = "New Relic Infrastructure Agent"
	apmInstrumentationName = "newrelic"
)

type agentInstrumentationApm struct {
	nrApp            *newrelic.Application
	harvester        *telemetry.Harvester
	hostnameResolver hostname.Resolver
}

func (a *agentInstrumentationApm) RecordMetric(ctx context.Context, metric metric) {
	var m telemetry.Metric
	metric = a.addHostName(metric)
	switch metric.Type {
	case Gauge:
		m = telemetry.Gauge{
			Timestamp: metric.Timestamp, Value: metric.Value, Name: metric.Name, Attributes: metric.Attributes,
		}
	case Sum:
		m = telemetry.Count{
			Timestamp: metric.Timestamp, Value: metric.Value, Name: metric.Name, Attributes: metric.Attributes,
		}
	case Histrogram:
		//not implemented?
		return
	}
	a.harvester.RecordMetric(m)
}

func (a *agentInstrumentationApm) StartTransaction(ctx context.Context, name string) (context.Context, Transaction) {
	nrTxn := a.nrApp.StartTransaction(name)
	txn := &TransactionApm{nrTxn: nrTxn}
	ctx = ContextWithTransaction(ctx, txn)

	return ctx, txn
}

func (a *agentInstrumentationApm) addHostName(metric metric) metric {
	if metric.Attributes == nil {
		metric.Attributes = make(map[string]interface{})
	}
	if _, ok := metric.Attributes["hostname"]; !ok {
		metric.Attributes["hostname"] = a.hostnameResolver.Long()
	}
	return metric
}

type TransactionApm struct {
	nrTxn *newrelic.Transaction
}

func (t *TransactionApm) AddAttribute(key string, value interface{}) {
	t.nrTxn.AddAttribute(key, value)
}

func (t *TransactionApm) StartSegment(ctx context.Context, name string) (context.Context, Segment) {
	return ctx, t.nrTxn.StartSegment(name)
}

func (t *TransactionApm) StartExternalSegment(ctx context.Context, name string, req *http.Request) (context.Context, Segment) {
	return ctx, newrelic.StartExternalSegment(t.nrTxn, req)
}

func (t *TransactionApm) NoticeError(err error) {
	t.nrTxn.NoticeError(err)
}

func (t *TransactionApm) End() {
	t.nrTxn.End()
}

type SegmentApm struct {
	nrSeg *newrelic.Segment
	ctx   context.Context
}

func (t *SegmentApm) AddAttribute(key string, value interface{}) {
	t.nrSeg.AddAttribute(key, value)
}

func (t *SegmentApm) End() {
	t.nrSeg.End()
}

func NewAgentInstrumentationApm(license string, apmEndpoint string, telemetryEndpoint string, resolver hostname.Resolver) (AgentInstrumentation, error) {
	nrApp, err := newrelic.NewApplication(
		newrelic.ConfigAppName(appName),
		newrelic.ConfigLicense(license),
		newrelic.ConfigDistributedTracerEnabled(true),
		func(c *newrelic.Config) {
			if apmEndpoint != "" {
				c.Host = apmEndpoint
			}
		},
		newrelic.ConfigFromEnvironment(),
		newrelic.ConfigLogger(apmLoggger{}),
	)
	if err != nil {
		return nil, err
	}

	harvester, err := telemetry.NewHarvester(
		telemetry.ConfigAPIKey(license),
		func(c *telemetry.Config) {
			if telemetryEndpoint != "" {
				c.MetricsURLOverride = telemetryEndpoint
			}
		},
	)
	if err != nil {
		return nil, err
	}

	return &agentInstrumentationApm{nrApp: nrApp, harvester: harvester, hostnameResolver: resolver}, nil
}

var aslog = log.WithComponent("AgentInstrumentation")

type apmLoggger struct {
}

func (A apmLoggger) Error(msg string, context map[string]interface{}) {
	ctx, err := json.Marshal(context)
	l := aslog
	if err == nil {
		l = aslog.WithField("context", string(ctx))
	}
	l.Error(msg)
}

func (A apmLoggger) Warn(msg string, context map[string]interface{}) {
	ctx, err := json.Marshal(context)
	l := aslog
	if err == nil {
		l = aslog.WithField("context", string(ctx))
	}
	l.Warn(msg)
}

func (A apmLoggger) Info(msg string, context map[string]interface{}) {
	ctx, err := json.Marshal(context)
	l := aslog
	if err == nil {
		l = aslog.WithField("context", string(ctx))
	}
	l.Info(msg)
}

func (A apmLoggger) Debug(msg string, context map[string]interface{}) {
	ctx, err := json.Marshal(context)
	l := aslog
	if err == nil {
		l = aslog.WithField("context", string(ctx))
	}
	l.Debug(msg)
}

func (A apmLoggger) DebugEnabled() bool {
	return logrus.IsLevelEnabled(logrus.DebugLevel)
}
