// Copyright 2019 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetryapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sync"
	"time"

	http2 "github.com/newrelic/infrastructure-agent/pkg/http"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/sirupsen/logrus"
)

// Harvester aggregates and reports metrics and spans.
type Harvester struct {
	// These fields are not modified after Harvester creation.  They may be
	// safely accessed without locking.
	config               Config
	commonAttributesJSON json.RawMessage

	// lock protects the mutable fields below.
	lock              sync.Mutex
	lastHarvest       time.Time
	rawMetrics        []Metric
	aggregatedMetrics map[metricIdentity]*metric
	spans             []Span
	commonAttributes  Attributes
	requestsQueue     chan request
	metricBatch       metricBatchHandler
	contextCancel     context.CancelFunc
}

const (
	// NOTE:  These constant values are used in Config field doc comments.
	defaultHarvestPeriod  = 5 * time.Second
	defaultHarvestTimeout = 15 * time.Second
	nrEntityID            = "nr.entity.id"
)

var (
	errAPIKeyUnset = errors.New("APIKey is required")
)

// NewHarvester creates a new harvester.
func NewHarvester(options ...func(*Config)) (*Harvester, error) {
	backgroundCtx, cancel := context.WithCancel(context.Background())
	cfg := Config{
		Client:                &http.Client{},
		HarvestPeriod:         defaultHarvestPeriod,
		HarvestTimeout:        defaultHarvestTimeout,
		MaxConns:              DefaultMaxConns,
		MaxEntitiesPerRequest: DefaultMaxEntitiesPerRequest,
		MaxEntitiesPerBatch:   DefaultMaxEntitiesPerBatch,
		Context:               backgroundCtx,
	}
	for _, opt := range options {
		opt(&cfg)
	}

	if cfg.APIKey == "" {
		cancel()
		return nil, errAPIKeyUnset
	}

	h := &Harvester{
		config:            cfg,
		lastHarvest:       time.Now(),
		aggregatedMetrics: make(map[metricIdentity]*metric),
		requestsQueue:     make(chan request, cfg.MaxConns),
		metricBatch:       newMetricBatchHandler(cfg.MaxEntitiesPerBatch),
		contextCancel:     cancel,
	}

	// Marshal the common attributes to JSON here to avoid doing it on every
	// harvest.  This also has the benefit that it avoids race conditions if
	// the consumer modifies the CommonAttributes map after calling
	// NewHarvester.
	if nil != h.config.CommonAttributes {
		attrs := vetAttributes(h.config.CommonAttributes, h.config.logError)
		attributesJSON, err := json.Marshal(attrs)
		if err != nil {
			h.config.logError(map[string]interface{}{
				"err":     err.Error(),
				"message": "error marshaling common attributes",
			})
		} else {
			h.commonAttributesJSON = attributesJSON
			h.commonAttributes = attrs
		}
		h.config.CommonAttributes = nil
	}

	h.config.logDebug(map[string]interface{}{
		"event":                  "harvester created",
		"harvest-period-seconds": h.config.HarvestPeriod.Seconds(),
		"metrics-url-override":   h.config.MetricsURLOverride,
		"spans-url-override":     h.config.SpansURLOverride,
		"version":                version,
	})

	defer h.Start()

	return h, nil
}

func (h *Harvester) Start() {

	if 0 != h.config.HarvestPeriod {
		go harvestRoutine(h)
	}

	for i := 1; i <= h.config.MaxConns; i++ {
		go func(workerNo int) {
			wlog := logger.WithField("WorkerNo.", workerNo)
			wlog.Debug("Starting worker.")
			for {
				select {
				case <-h.config.Context.Done():
					wlog.Debug("Shutting down worker.")
					return
				case req := <-h.requestsQueue:
					harvestRequest(req, &h.config)
				}
			}
		}(i)
	}
}

var (
	errSpanIDUnset  = errors.New("span id must be set")
	errTraceIDUnset = errors.New("trace id must be set")
)

// RecordSpan records the given span.
func (h *Harvester) RecordSpan(s Span) error {
	if nil == h {
		return nil
	}
	if "" == s.TraceID {
		return errTraceIDUnset
	}
	if "" == s.ID {
		return errSpanIDUnset
	}
	if s.Timestamp.IsZero() {
		s.Timestamp = time.Now()
	}

	h.lock.Lock()
	defer h.lock.Unlock()

	h.spans = append(h.spans, s)
	return nil
}

func (h *Harvester) Cancel() {
	h.contextCancel()
}

// RecordMetric adds a fully formed metric.  This metric is not aggregated with
// any other metrics and is never dropped.  The Timestamp field must be
// specified on Gauge metrics.  The Timestamp/Interval fields on Count and
// Summary are optional and will be assumed to be the harvester batch times if
// unset.  Use MetricAggregator() instead to aggregate metrics.
func (h *Harvester) RecordMetric(m Metric) {
	if nil == h {
		return
	}
	h.lock.Lock()
	defer h.lock.Unlock()

	if fields := m.validate(); nil != fields {
		h.config.logError(fields)
		return
	}

	h.rawMetrics = append(h.rawMetrics, m)
}

func (h *Harvester) RecordInfraMetrics(commonAttributes Attributes, metrics []Metric) error {
	if nil == h {
		return nil
	}

	for i := range metrics {
		if fields := metrics[i].validate(); nil != fields {
			h.config.logError(fields)
			return errors.New("invalid error") // todo figure out which one broke
		}
	}

	var attributesJSON json.RawMessage
	var identity string

	if len(commonAttributes) < 1 {
		attributesJSON = h.commonAttributesJSON
	}

	if len(commonAttributes) > 0 {
		for k, v := range h.commonAttributes {
			if _, ok := commonAttributes[k]; !ok {
				commonAttributes[k] = v
			}
		}

		attrs := vetAttributes(commonAttributes, h.config.logError)
		var errJSON error
		attributesJSON, errJSON = json.Marshal(attrs)
		if errJSON != nil {
			h.config.logError(map[string]interface{}{
				"err":     errJSON.Error(),
				"message": "error marshaling common attributes",
			})
			logger.WithError(errJSON).Warn("Setting default common attributes")
			attributesJSON = h.commonAttributesJSON
		} else {
			identity, _ = commonAttributes[nrEntityID].(string)
		}
	}

	return h.metricBatch.enqueue(metricBatch{
		Identity:       identity,
		AttributesJSON: attributesJSON,
		Metrics:        metrics,
	})
}

type response struct {
	statusCode int
	body       []byte
	err        error
	retryAfter string
}

var (
	backoffSequenceSeconds = []int{0, 1, 2, 4, 8, 16}
)

func (r response) needsRetry(_ *Config, attempts int) (bool, time.Duration) {
	if attempts >= len(backoffSequenceSeconds) {
		attempts = len(backoffSequenceSeconds) - 1
	}
	backoff := time.Duration(backoffSequenceSeconds[attempts]) * time.Second

	switch r.statusCode {
	case 202, 200:
		// success
		return false, 0
	case 400, 403, 404, 405, 411, 413:
		// errors that should not retry
		return false, 0
	case 429:
		// special retry backoff time
		if "" != r.retryAfter {
			// Honor Retry-After header value in seconds
			if d, err := time.ParseDuration(r.retryAfter + "s"); nil == err {
				if d > backoff {
					return true, d
				}
			}
		}
		return true, backoff
	default:
		// all other errors should retry
		return true, backoff
	}
}

func postData(req *http.Request, client *http.Client) response {
	if log.IsLevelEnabled(logrus.TraceLevel) {
		req = http2.WithTracer(req, "harvester")
	}

	resp, err := client.Do(req)
	if nil != err {
		return response{err: fmt.Errorf("error posting data: %v", err)}
	}
	defer resp.Body.Close()

	r := response{
		statusCode: resp.StatusCode,
		retryAfter: resp.Header.Get("Retry-After"),
	}

	// On success, metrics ingest returns 202, span ingest returns 200.
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted {
		r.body, _ = io.ReadAll(resp.Body)
	} else {
		_, _ = io.ReadAll(resp.Body)
		r.err = fmt.Errorf("unexpected post response code: %d: %s",
			resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	return r
}

func (h *Harvester) swapOutMetrics(ctx context.Context, now time.Time) []request {
	h.lock.Lock()
	lastHarvest := h.lastHarvest
	h.lastHarvest = now
	rawMetrics := h.rawMetrics
	h.rawMetrics = nil
	aggregatedMetrics := h.aggregatedMetrics
	h.aggregatedMetrics = make(map[metricIdentity]*metric, len(aggregatedMetrics))
	h.lock.Unlock()

	for _, m := range aggregatedMetrics {
		if nil != m.c {
			rawMetrics = append(rawMetrics, m.c)
		}
		if nil != m.s {
			rawMetrics = append(rawMetrics, m.s)
		}
		if nil != m.g {
			rawMetrics = append(rawMetrics, m.g)
		}
	}

	if 0 == len(rawMetrics) {
		return nil
	}

	batch := &metricBatch{
		Timestamp:      lastHarvest,
		Interval:       now.Sub(lastHarvest),
		AttributesJSON: h.commonAttributesJSON,
		Metrics:        rawMetrics,
	}
	reqs, err := newRequests(ctx, batch, h.config.APIKey, h.config.metricURL(), h.config.userAgent())
	if nil != err {
		h.config.logError(map[string]interface{}{
			"err":     err.Error(),
			"message": "error creating requests for metrics",
		})
		return nil
	}
	return reqs
}

func (h *Harvester) swapOutBatchMetrics(ctx context.Context) (req []request) {
	h.lock.Lock()
	rawMetricsBatch := h.metricBatch.dequeue()
	h.lock.Unlock()
	var err error
	r := config{
		rawMetricsBatch,
		h.config.APIKey,
		h.config.metricURL(),
		h.config.userAgent(),
		h.config.MaxEntitiesPerRequest,
	}
	req, err = newBatchRequest(ctx, r)
	if err != nil {
		h.config.logError(map[string]interface{}{
			"err":     err.Error(),
			"message": "error creating requests for batch metric",
		})
	}
	return req
}

func (h *Harvester) swapOutSpans(ctx context.Context) []request {
	h.lock.Lock()
	sps := h.spans
	h.spans = nil
	h.lock.Unlock()

	if nil == sps {
		return nil
	}
	batch := &spanBatch{
		AttributesJSON: h.commonAttributesJSON,
		Spans:          sps,
	}
	reqs, err := newRequests(ctx, batch, h.config.APIKey, h.config.spanURL(), h.config.userAgent())
	if nil != err {
		h.config.logError(map[string]interface{}{
			"err":     err.Error(),
			"message": "error creating requests for spans",
		})
		return nil
	}
	return reqs
}

func harvestRequest(req request, cfg *Config) {
	var attempts int
	for {
		cfg.logDebug(map[string]interface{}{
			"event":       "data post",
			"url":         req.Request.URL.String(),
			"body-length": req.compressedBodyLength,
		})
		// Check if the audit log is enabled to prevent unnecessarily
		// copying UncompressedBody.
		if cfg.auditLogEnabled() {
			cfg.logAudit(map[string]interface{}{
				"event":   "uncompressed request body",
				"url":     req.Request.URL.String(),
				"data":    jsonString(req.UncompressedBody),
				"headers": req.Request.Header,
			})
		}

		resp := postData(req.Request, cfg.Client)

		if nil != resp.err {
			cfg.logError(map[string]interface{}{
				"err": resp.err.Error(),
			})
		} else {
			cfg.logDebug(map[string]interface{}{
				"event":  "data post response",
				"status": resp.statusCode,
				"body":   jsonOrString(resp.body),
			})
		}
		retry, backoff := resp.needsRetry(cfg, attempts)
		if !retry {
			return
		}

		tmr := time.NewTimer(backoff)
		select {
		case <-tmr.C:
			break
		case <-req.Request.Context().Done():
			tmr.Stop()
			return
		}
		attempts++

		// Reattach request body because the original one has already been read
		// and closed.
		req.Request.Body = io.NopCloser(bytes.NewBuffer(req.compressedBody))
	}
}

// HarvestNow sends metric and span data to New Relic.  This method blocks until
// all data has been sent successfully or the Config.HarvestTimeout timeout has
// elapsed. This method can be used with a zero Config.HarvestPeriod value to
// control exactly when data is sent to New Relic servers.
func (h *Harvester) HarvestNow(ct context.Context) {
	if nil == h {
		return
	}

	ctx, _ := context.WithTimeout(ct, h.config.HarvestTimeout)

	var reqs []request
	reqs = append(reqs, h.swapOutMetrics(ctx, time.Now())...)
	reqs = append(reqs, h.swapOutSpans(ctx)...)
	reqs = append(reqs, h.swapOutBatchMetrics(ctx)...)

	for _, req := range reqs {
		h.requestsQueue <- req
		if err := ctx.Err(); err != nil {
			// NOTE: It is possible that the context was
			// cancelled/timedout right after the request
			// successfully finished.  In that case, we will
			// erroneously log a message.  I (will) don't think
			// that's worth trying to engineer around.
			h.config.logError(map[string]interface{}{
				"event":         "harvest cancelled or timed out",
				"message":       "dropping data",
				"context-error": err.Error(),
			})
			return
		}
	}
}

func minDuration(d1, d2 time.Duration) time.Duration {
	if d1 < d2 {
		return d1
	}
	return d2
}

func harvestRoutine(h *Harvester) {
	// Introduce a small jitter to ensure the backend isn't hammered if many
	// harvesters start at once.
	d := minDuration(h.config.HarvestPeriod, 3*time.Second)
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	jitter := time.Nanosecond * time.Duration(rnd.Int63n(d.Nanoseconds()))
	time.Sleep(jitter)

	ticker := time.NewTicker(h.config.HarvestPeriod)
	for {
		select {
		case <-ticker.C:
			go h.HarvestNow(h.config.Context)
		case <-h.config.Context.Done():
			return
		}
	}
}

type metricIdentity struct {
	// Note that the type is not a field here since a single 'metric' type
	// may contain a count, gauge, and summary.
	Name           string
	attributesJSON string
}

type metric struct {
	s *Summary
	c *Count
	g *Gauge
}

type metricHandle struct {
	metricIdentity
	harvester *Harvester
}

func newMetricHandle(h *Harvester, name string, attributes map[string]interface{}) metricHandle {

	return metricHandle{
		harvester: h,
		metricIdentity: metricIdentity{
			attributesJSON: string(MarshalOrderedAttributes(attributes)),
			Name:           name,
		},
	}
}

// findOrCreateMetric finds or creates the metric associated with the given
// identity.  This function assumes the Harvester is locked.
func (h *Harvester) findOrCreateMetric(identity metricIdentity) *metric {
	m := h.aggregatedMetrics[identity]
	if nil == m {
		// this happens the first time we update the value,
		// or after a harvest when the metric is removed.
		m = &metric{}
		h.aggregatedMetrics[identity] = m
	}
	return m
}

// MetricAggregator is used to aggregate individual data points into metrics.
type MetricAggregator struct {
	harvester *Harvester
}

// MetricAggregator returns a metric aggregator.  Use this instead of
// RecordMetric if you have individual data points that you would like to
// combine into metrics.
func (h *Harvester) MetricAggregator() *MetricAggregator {
	if nil == h {
		return nil
	}
	return &MetricAggregator{harvester: h}
}

// Count creates a new AggregatedCount metric.
func (ag *MetricAggregator) Count(name string, attributes map[string]interface{}) *AggregatedCount {
	if nil == ag {
		return nil
	}
	return &AggregatedCount{metricHandle: newMetricHandle(ag.harvester, name, attributes)}
}

// Gauge creates a new AggregatedGauge metric.
func (ag *MetricAggregator) Gauge(name string, attributes map[string]interface{}) *AggregatedGauge {
	if nil == ag {
		return nil
	}
	return &AggregatedGauge{metricHandle: newMetricHandle(ag.harvester, name, attributes)}
}

// Summary creates a new AggregatedSummary metric.
func (ag *MetricAggregator) Summary(name string, attributes map[string]interface{}) *AggregatedSummary {
	if nil == ag {
		return nil
	}
	return &AggregatedSummary{metricHandle: newMetricHandle(ag.harvester, name, attributes)}
}

type metricBatchHandler struct {
	lock  sync.Mutex
	index int
	queue []metricBatch
}

func newMetricBatchHandler(maxDepth int) metricBatchHandler {
	return metricBatchHandler{
		index: 0,
		queue: make([]metricBatch, maxDepth),
	}
}

func (m *metricBatchHandler) enqueue(metric metricBatch) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.index == cap(m.queue) {
		return errors.New("could not queue metric: queue is full")
	}

	m.queue[m.index] = metric
	m.index++
	return nil
}

func (m *metricBatchHandler) dequeue() []metricBatch {
	m.lock.Lock()
	defer m.lock.Unlock()

	res := m.queue[:m.index]
	m.queue = make([]metricBatch, cap(m.queue))
	m.index = 0
	return res
}
