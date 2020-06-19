// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package agent

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/sirupsen/logrus"

	"github.com/newrelic/infrastructure-agent/pkg/backend/backoff"

	"github.com/newrelic/infrastructure-agent/internal/agent/id"

	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/sample"

	backendhttp "github.com/newrelic/infrastructure-agent/pkg/backend/http"
	"github.com/newrelic/infrastructure-agent/pkg/config"
)

const (
	EVENT_QUEUE_CAPACITY       = 1000
	BATCH_QUEUE_CAPACITY       = 200 // Queue memory consumption cCould be a MAX of config.MaxMetricsBatchSizeBytes * BATCH_QUEUE_CAPACITY in size
	MAX_EVENT_BATCH_COUNT      = 500
	EVENT_BATCH_TIMER_DURATION = 1 // seconds, How often we will queue batches of events even if we haven't hit max batch size
)

var ilog = log.WithComponent("MetricsIngestSender")

type eventData struct {
	entityKey entity.Key // Unique identifier of the entity for this event
	entityID  entity.ID
	agentKey  string
	data      json.RawMessage // Pre-marshalled JSON data for a single event.
}

type eventBatch []eventData // A collection of pre-marshalled event JSON objects.

// IsAgent returns true when event belongs to the agent/local entity.
func (d *eventData) IsAgent() bool {
	return d.entityKey.String() == d.agentKey
}

// eventSender specifies a type of object which can take events to eventually send to <somewhere>.
// The send operation is assumed to be asynchronous, and so this only supports an instruction to
// queue an event for later sending.
type eventSender interface {
	QueueEvent(event sample.Event, entity entity.Key) error // Queues the given event data for the entity identified by the given "entity" string
	Start() error
	Stop() error
}

// Implementation of eventSender which periodically sends events to the metrics ingest endpoint.
type metricsIngestSender struct {
	eventQueue               chan eventData  // Individual events waiting to be put into a batch
	batchQueue               chan eventBatch // Event batches which are ready to be sent
	metricIngestURL          string
	internalRoutineWaits     *sync.WaitGroup // Waitgroup to keep track of how many goroutines are running and wait for them to stop
	stopChannel              chan bool       // Channel will be closed when we want to stop all internal goroutines
	licenseKey               string
	userAgent                string
	HttpClient               backendhttp.Client
	Context                  *context
	sendErrorCount           uint32
	sendBackoffMax           uint32
	maxMetricsBatchSizeBytes int
	agentIDProvide           id.Provide
	connectEnabled           bool
	getBackoffTimer          func(time.Duration) *time.Timer
	postCount                uint64 // counts post requests for debugging purposes
}

func newMetricsIngestSender(ctx *context, licenseKey, userAgent string, httpClient backendhttp.Client, connectEnabled bool) *metricsIngestSender {
	cfg := ctx.Config()

	metricIngestURL := fmt.Sprintf("%s/%s", ctx.Config().CollectorURL,
		strings.TrimPrefix(cfg.MetricsIngestEndpoint, "/"))

	if os.Getenv("DEV_METRICS_INGEST_URL") != "" {
		metricIngestURL = os.Getenv("DEV_METRICS_INGEST_URL")
	}
	metricIngestURL = strings.TrimSuffix(metricIngestURL, "/")
	eventQueue := EVENT_QUEUE_CAPACITY
	if cfg.EventQueueDepth > eventQueue {
		eventQueue = cfg.EventQueueDepth
	}

	batchQueue := BATCH_QUEUE_CAPACITY
	if cfg.BatchQueueDepth > batchQueue {
		batchQueue = cfg.BatchQueueDepth
	}

	maxMetricsBatchSizeBytes := cfg.MaxMetricsBatchSizeBytes
	if maxMetricsBatchSizeBytes <= 0 || maxMetricsBatchSizeBytes > config.DefaultMaxMetricsBatchSizeBytes {
		maxMetricsBatchSizeBytes = config.DefaultMaxMetricsBatchSizeBytes
	}

	return &metricsIngestSender{
		eventQueue:               make(chan eventData, eventQueue),
		batchQueue:               make(chan eventBatch, batchQueue),
		metricIngestURL:          metricIngestURL,
		internalRoutineWaits:     &sync.WaitGroup{},
		licenseKey:               licenseKey,
		userAgent:                userAgent,
		Context:                  ctx,
		sendBackoffMax:           config.MAX_BACKOFF,
		maxMetricsBatchSizeBytes: maxMetricsBatchSizeBytes,
		HttpClient:               httpClient,
		agentIDProvide:           ctx.AgentIdentity,
		connectEnabled:           connectEnabled,
		getBackoffTimer:          time.NewTimer,
		postCount:                0,
	}
}

func (sender *metricsIngestSender) Debug() bool {
	return sender.Context.Config().Debug
}

// Start a couple of background routines to handle incoming data and post it to the server periodically.
func (sender *metricsIngestSender) Start() (err error) {
	if sender.stopChannel != nil {
		return fmt.Errorf("Cannot start sender: The sender is already running. (stopChannel is not nil)")
	}

	// Set up the stop channel so the routines can wait for it to be closed
	sender.stopChannel = make(chan bool)

	// Wait for accumulateBatches and sendBatches to complete
	sender.internalRoutineWaits.Add(2)

	go func() {
		defer sender.internalRoutineWaits.Done()
		sender.accumulateBatches()
	}()

	go func() {
		defer sender.internalRoutineWaits.Done()
		sender.sendBatches()
	}()

	return
}

// Stop will gracefully shut down all sending processes and reset the state of the sender.
// After Stop() returns, it is safe to call Start() again on the same sender instance.
func (sender *metricsIngestSender) Stop() (err error) {
	if sender.stopChannel == nil {
		return fmt.Errorf("Cannot stop sender: The sender is not running. (stopChannel is nil)")
	}

	close(sender.stopChannel)
	sender.internalRoutineWaits.Wait()
	sender.stopChannel = nil

	return
}

// We can accept any kind of object to represent an event. We assume that it will marshal to a valid JSON event object.
func (sender *metricsIngestSender) QueueEvent(event sample.Event, key entity.Key) (err error) {
	agentKey := sender.Context.AgentIdentifier()
	// Default to the agent's own ID if we didn't receive one
	if key == "" {
		key = entity.Key(agentKey)
	}
	event.Entity(key)

	edata, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("error marshalling event to JSON: %+v (%+v)", event, err)
	}

	if len(edata) > sender.maxMetricsBatchSizeBytes {
		return fmt.Errorf("Could not queue event: Event is larger than the maximum event post size (%d > %d).", len(edata), sender.maxMetricsBatchSizeBytes)
	}

	queuedEvent := eventData{
		entityKey: key,
		data:      edata,
		agentKey:  agentKey,
	}

	select {
	case sender.eventQueue <- queuedEvent:
		return nil
	default:
		return fmt.Errorf("Could not queue event: Queue is full.")
	}
}

// Collect events from the queue and accumulate them into batches which can be sent up to metrics ingest.
// This hands off each batch to another channel which is consumed by a routine which does the actual post.
// We send batches on a timer or on accumulating some number of events in the current batch.
//
// We have a two-channel approach (event -> eventQueue -> batch -> batchQueue -> HTTP post) to minimize
// the impact of high-latency HTTP calls. If HTTP calls are slow, we'll still be able to run the event
// queue receiver and accumulate a reasonable number of batches before we fill up on batches as well.
func (sender *metricsIngestSender) accumulateBatches() {
	var batch eventBatch
	var batchBytes int // Accumulated batch size in bytes

	sendTimerD := EVENT_BATCH_TIMER_DURATION * time.Second
	sendTimer := time.NewTimer(sendTimerD)
	for {
		select {
		case event := <-sender.eventQueue:

			// Add entityID if connect is enabled and if is not a remote entity.
			if sender.connectEnabled && event.IsAgent() {
				event.entityID = sender.agentIDProvide().ID
			}

			if batchBytes+len(event.data) > sender.maxMetricsBatchSizeBytes || len(batch) == MAX_EVENT_BATCH_COUNT {
				// Current batch + this event would either be too many events or too many bytes, so queue the batch first.
				select {
				case sender.batchQueue <- batch:
					batch = make(eventBatch, 0)
					batchBytes = 0
				case <-sender.stopChannel:
					return
				}
			}
			batch = append(batch, event)
			batchBytes += len(event.data)
		case <-sendTimer.C:
			// Timer has fired - send any queued events to ensure a minimum delay in sending.
			if len(batch) > 0 {
				select {
				case sender.batchQueue <- batch:
					batch = make(eventBatch, 0)
					batchBytes = 0
				case <-sender.stopChannel:
					return
				}
			}
			sendTimer.Reset(sendTimerD)
		case <-sender.stopChannel:
			// Stop channel has been closed - exit.
			// There might still be some events in the queue, but they'll still be there in case we start the sender back up.
			return
		}
	}
}

// MetricPost entity item for the HTTP post to be sent to the ingest service.
type MetricPost struct {
	ExternalKeys []string          `json:"ExternalKeys,omitempty"`
	EntityID     entity.ID         `json:"EntityID,omitempty"`
	IsAgent      bool              `json:"IsAgent"`
	Events       []json.RawMessage `json:"Events"`
	// Entity ID of the reporting agent, which will = EntityID when IsAgent == true.
	// The field is required in the backend for host metadata matching of the remote entities
	ReportingAgentID entity.ID `json:"ReportingAgentID,omitempty"`
}

// MetricPostBatch HTTP post batching all the MetricPost per entity to be sent to the ingest service.
type MetricPostBatch []*MetricPost

func newMetricPost(entityKey entity.Key, entityID, agentID entity.ID, agentKey string) *MetricPost {
	mp := &MetricPost{
		IsAgent: entityKey.String() == agentKey,
	}

	if !entityID.IsEmpty() {
		mp.EntityID = entityID
	} else {
		// ExternalKeys are still required in Connect V1 for remote entities. (Will be removed when register added)
		mp.ExternalKeys = []string{entityKey.String()}
	}

	mp.ReportingAgentID = agentID

	return mp
}

// getLoggingField will add an identifier for the MetricPost for the logs.
func (mp *MetricPost) getLoggingField() logrus.Fields {
	if len(mp.ExternalKeys) > 0 {
		return logrus.Fields{"id": mp.ExternalKeys[0]}
	}
	return logrus.Fields{"key": mp.EntityID.String()}
}

// return, as a logging field, the different timestamps of the post metrics
func (mp *MetricPost) getTimestampLoggingFields() logrus.Fields {
	tsSet := map[int64]struct{}{}
	timestamps := []string{}
	for _, event := range mp.Events {
		var timestamp struct {
			Value int64 `json:"timestamp"`
		}
		if err := json.Unmarshal(event, &timestamp); err != nil || timestamp.Value == 0 {
			continue
		}
		if _, ok := tsSet[timestamp.Value]; ok {
			continue
		}
		tsSet[timestamp.Value] = struct{}{}

		timestamps = append(timestamps, time.Unix(timestamp.Value, 0).String())
	}
	return logrus.Fields{"timestamps": timestamps}
}

// Wait for queued batches and send any to the ingest API
func (sender *metricsIngestSender) sendBatches() {
	retryBO := backoff.NewDefaultBackoff()
	for {
		select {

		case batch := <-sender.batchQueue:
			pclog := ilog.WithField("postCount", sender.postCount)
			sender.postCount++

			agentKey := ""
			dataByEntity := make(map[entity.Key]*MetricPost)

			agentID := sender.agentID()

			// We need to rebuild the array of events as a []json.RawMessage, or else JSON marshalling won't handle them correctly.
			for _, event := range batch {
				entityData := dataByEntity[event.entityKey]
				if entityData == nil {
					entityData = newMetricPost(event.entityKey, event.entityID, agentID, event.agentKey)
					dataByEntity[event.entityKey] = entityData
				}
				entityData.Events = append(entityData.Events, event.data)
				if event.agentKey != "" {
					agentKey = event.agentKey
				}
			}

			var bulkPost MetricPostBatch
			for _, entityData := range dataByEntity {

				pclog.WithFieldsF(entityData.getLoggingField).
					WithFieldsF(entityData.getTimestampLoggingFields).
					WithField("numEvents", len(entityData.Events)).
					Debug("Sending events to metrics-ingest.")
				bulkPost = append(bulkPost, entityData)
			}

			pclog.Debug("Preparing metrics post.")

			err := sender.doPost(bulkPost, agentKey)

			if err == nil {
				pclog.Debug("Metrics post succeeded.")
				sender.sendErrorCount = 0
				retryBO.Reset()
				continue
			}

			sender.sendErrorCount++
			pclog.WithError(err).WithField("sendErrorCount", sender.sendErrorCount).Error("metric sender can't process")

			e, ok := err.(*errRetry)
			if !ok {
				continue
			}

			if e.retryPolicy.After > 0 {
				pclog.WithField("retryAfter", e.retryPolicy.After).Debug("Metric sender retry requested.")
				retryBO.Reset()
				sender.backoff(e.retryPolicy.After)
				continue
			}
			retryBOAfter := retryBO.DurationWithMax(e.retryPolicy.MaxBackOff)
			pclog.WithField("retryBackoffAfter", retryBOAfter).Debug("Metric sender backoff and retry requested.")
			sender.backoff(retryBOAfter)
		case <-sender.stopChannel:
			// Stop channel has been closed - exit.
			// There might still be some batches in the queue, but they'll still be there in case we start the sender back up.
			return
		}
	}
}

func (s *metricsIngestSender) agentID() entity.ID {
	if s.Context != nil &&
		s.Context.Config() != nil &&
		s.Context.Config().ConnectEnabled {

		return s.Context.AgentID()
	}
	return entity.EmptyID
}

// backoff waits for the specified duration or a signal from the stop
// channel, whichever happens first.
func (s *metricsIngestSender) backoff(d time.Duration) {
	backoffTimer := s.getBackoffTimer(d)
	select {
	case <-s.stopChannel:
	case <-backoffTimer.C:
	}
}

// Make one HTTP call to push a load of events up to the server
func (sender *metricsIngestSender) doPost(post []*MetricPost, agentKey string) error {
	if agentKey == "" {
		ilog.Warn("no available agent-id on metrics sender")
	}

	postBytes, err := json.Marshal(post)
	if err != nil {
		return fmt.Errorf("Could not marshal events object [%v]: %v", post, err)
	}

	// GZIP
	var reqBuf *bytes.Buffer
	if sender.Context.Config().PayloadCompressionLevel > gzip.NoCompression {
		// GZIP
		reqBuf = &bytes.Buffer{}
		compressionLevel := sender.Context.Config().PayloadCompressionLevel
		gzipWriter, err := gzip.NewWriterLevel(reqBuf, compressionLevel)
		if err != nil {
			return fmt.Errorf("Unable to create gzip writer: %v", err)
		}
		if _, err := gzipWriter.Write(postBytes); err != nil {
			return fmt.Errorf("Gzip writer was not able to write to request body: %s", err)
		}
		if err := gzipWriter.Close(); err != nil {
			return fmt.Errorf("Gzip writer did not close: %s", err)
		}
	} else {
		reqBuf = bytes.NewBuffer(postBytes)
	}
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/events/bulk", sender.metricIngestURL), reqBuf)
	if err != nil {
		return fmt.Errorf("Error creating event POST: %v", err)
	}

	if sender.Context.Config().PayloadCompressionLevel > gzip.NoCompression {
		req.Header.Set("Content-Encoding", "gzip")
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", sender.userAgent)
	req.Header.Set(backendhttp.LicenseHeader, sender.licenseKey)

	req.Header.Set(backendhttp.EntityKeyHeader, agentKey)
	if sender.connectEnabled {
		agentID := sender.Context.AgentID()
		if agentID.IsEmpty() {
			return fmt.Errorf("empty agent-id on metrics sender")
		}
		req.Header.Set(backendhttp.AgentEntityIdHeader, agentID.String())
	}

	resp, err := sender.HttpClient(req)
	if err != nil {
		return fmt.Errorf("error sending events: %v", err)
	}

	// To let the http client reusing the connections, the response body
	// must be completely read and closed.
	// Not reusing the connections may lead to metrics being accumulated
	// and submitted with long delay in systems where the connection
	// establishment is slow.
	defer resp.Body.Close()
	buf, bodyErr := ioutil.ReadAll(resp.Body)

	hasError, cause := backendhttp.IsResponseUnsuccessful(resp)
	if !hasError {
		return nil
	}

	if bodyErr != nil {
		return fmt.Errorf("error sending events: Unable to read server response: %s", bodyErr)
	}

	var retryPolicy backendhttp.RetryPolicy
	retryAfterH := resp.Header.Get("Retry-After")
	if retryAfterH != "" {
		if retryPolicy.After, err = time.ParseDuration(retryAfterH + "s"); err != nil {
			ilog.WithError(err).Debug(
				"error parsing connect Retry-After header, continuing with exponential backoff",
			)
		}
	}

	retryPolicy.MaxBackOff = backoff.GetMaxBackoffByCause(cause)

	return newErrRetry(
		"events were not accepted",
		resp.StatusCode,
		resp.Status,
		string(buf),
		retryPolicy,
	)
}
