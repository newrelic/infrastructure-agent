// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package agent

import (
	"bytes"
	"compress/gzip"
	context2 "context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/newrelic/infrastructure-agent/pkg/log"

	"github.com/newrelic/infrastructure-agent/internal/agent/id"
	"github.com/newrelic/infrastructure-agent/pkg/backend/backoff"
	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/sample"

	backendhttp "github.com/newrelic/infrastructure-agent/pkg/backend/http"
	"github.com/newrelic/infrastructure-agent/pkg/backend/inventoryapi"
	"github.com/newrelic/infrastructure-agent/pkg/config"
)

var vlog = log.WithComponent("VortexEventSender")

// MetricPost entity item for the HTTP post to be sent to the ingest service.
type MetricVortexPost struct {
	EntityID  entity.ID         `json:"EntityID"`
	EntityKey entity.Key        `json:"EntityKey"`
	IsAgent   bool              `json:"IsAgent"`
	Events    []json.RawMessage `json:"Events"`
	// Entity ID of the reporting agent, which will = EntityID when IsAgent == true.
	// The field is required in the backend for host metadata matching of the remote entities
	ReportingAgentID entity.ID `json:"ReportingAgentID"`
}

// MetricPostBatch HTTP post batching all the MetricPost per entity to be sent to the ingest service.
type MetricVortexPostBatch []*MetricVortexPost

type eventVortexData struct {
	entityKey entity.Key
	entityID  entity.ID
	agentKey  string
	data      json.RawMessage // Pre-marshalled JSON data for a single event.
}
type eventVortexBatch []eventVortexData // A collection of pre-marshalled event JSON objects.

type errRetry struct {
	*inventoryapi.IngestError
	retryPolicy backendhttp.RetryPolicy
}

func newErrRetry(msg string, code int, status, body string, retryPolicy backendhttp.RetryPolicy) *errRetry {
	err := inventoryapi.NewIngestError(msg, code, status, body)
	return &errRetry{
		IngestError: err,
		retryPolicy: retryPolicy,
	}
}

// Implementation of eventSender which periodically sends events to the metrics ingest endpoint.
type vortexEventSender struct {
	eventQueue               chan eventVortexData // Individual events waiting to be put into a batch
	eventsWithID             chan eventVortexData
	eventsWithoutID          chan eventVortexData
	batchQueue               chan eventVortexBatch // Event batches which are ready to be sent
	metricIngestURL          string
	internalRoutineWaits     *sync.WaitGroup // Waitgroup to keep track of how many goroutines are running and wait for them to stop
	stopChannel              chan bool       // Channel will be closed when we want to stop all internal goroutines
	licenseKey               string
	userAgent                string
	HttpClient               backendhttp.Client
	Context                  *context
	sendErrorCount           *uint32
	sendBackoffMax           uint32
	maxMetricsBatchSizeBytes int
	agentIDProvide           id.Provide
	provideIDs               ProvideIDs
	localEntityMap           entity.KnownIDs
	registerWorkers          int
	registerBatchSize        int
	registerFrequency        time.Duration
	getBackoffTimer          func(time.Duration) *time.Timer
}

// IsAgent returns true when event belongs to the agent/local entity.
func (d *eventVortexData) IsAgent() bool {
	return d.entityKey.String() == d.agentKey
}

func newVortexEventSender(ctx *context, licenseKey, userAgent string, httpClient backendhttp.Client, provideIDs ProvideIDs, localEntityMap entity.KnownIDs) eventSender {
	cfg := ctx.Config()

	metricIngestURL := fmt.Sprintf("%s/%s",
		strings.TrimSuffix(cfg.CollectorURL, "/"),
		strings.TrimPrefix(cfg.MetricsIngestEndpoint, "/"))
	if os.Getenv("DEV_METRICS_INGEST_URL") != "" {
		metricIngestURL = os.Getenv("DEV_METRICS_INGEST_URL")
	}
	metricIngestURL = strings.TrimSuffix(metricIngestURL, "/")

	// Nice2Have: extract this config construction to config package
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

	return &vortexEventSender{
		eventQueue:               make(chan eventVortexData, eventQueue),
		eventsWithID:             make(chan eventVortexData, eventQueue),
		eventsWithoutID:          make(chan eventVortexData, eventQueue),
		batchQueue:               make(chan eventVortexBatch, batchQueue),
		metricIngestURL:          metricIngestURL,
		internalRoutineWaits:     &sync.WaitGroup{},
		licenseKey:               licenseKey,
		userAgent:                userAgent,
		Context:                  ctx,
		sendBackoffMax:           config.MAX_BACKOFF,
		maxMetricsBatchSizeBytes: maxMetricsBatchSizeBytes,
		HttpClient:               httpClient,
		agentIDProvide:           ctx.AgentIdentity,
		provideIDs:               provideIDs,
		localEntityMap:           localEntityMap,
		registerWorkers:          cfg.RegisterConcurrency,
		registerBatchSize:        cfg.RegisterBatchSize,
		registerFrequency:        time.Duration(cfg.RegisterFrequencySecs) * time.Second,
		getBackoffTimer:          time.NewTimer,
		sendErrorCount:           new(uint32),
	}
}

func (s *vortexEventSender) Debug() bool {
	return s.Context.Config().Debug
}

// Start a couple of background routines to handle incoming data and post it to the server periodically.
func (s *vortexEventSender) Start() (err error) {
	if s.stopChannel != nil {
		return fmt.Errorf("Cannot start sender: The sender is already running. (stopChannel is not nil)")
	}

	// Set up the stop channel so the routines can wait for it to be closed
	s.stopChannel = make(chan bool)

	go func() {
		s.internalRoutineWaits.Add(1)
		defer s.internalRoutineWaits.Done()
		s.accumulateBatches()
	}()

	go func() {
		s.internalRoutineWaits.Add(1)
		defer s.internalRoutineWaits.Done()
		s.sendBatches()
	}()

	return
}

// Stop will gracefully shut down all sending processes and reset the state of the sender.
// After Stop() returns, it is safe to call Start() again on the same sender instance.
func (s *vortexEventSender) Stop() (err error) {
	if s.stopChannel == nil {
		return fmt.Errorf("Cannot stop sender: The sender is not running. (stopChannel is nil)")
	}
	if s.eventsWithoutID == nil {
		return fmt.Errorf("Cannot stop sender: The sender is not running. (eventsWithoutID is nil)")
	}
	close(s.stopChannel)
	s.internalRoutineWaits.Wait()
	s.stopChannel = nil

	return
}

// We can accept any kind of object to represent an event. We assume that it will marshal to a valid JSON event object.
func (s *vortexEventSender) QueueEvent(event sample.Event, key entity.Key) (err error) {
	agentKey := s.Context.AgentIdentifier()
	// Default to the agent's own ID if we didn't receive one
	if key == "" {
		key = entity.Key(agentKey)
	}
	event.Entity(key)

	edata, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("error marshalling event to JSON: %+v (%+v)", event, err)
	}

	if len(edata) > s.maxMetricsBatchSizeBytes {
		return fmt.Errorf("cannot queue event: larger than max size (%d > %d)", len(edata), s.maxMetricsBatchSizeBytes)
	}

	select {
	case s.eventQueue <- newEventData(key, edata, agentKey):
	default:
		err = fmt.Errorf("cannot queue event: full queue, ev: %s", key)
	}
	return
}

func newEventData(key entity.Key, edata []byte, agentKey string) eventVortexData {
	return eventVortexData{
		// entityID will be set afterwards on async manner
		entityKey: key,
		data:      edata,
		agentKey:  agentKey,
	}
}

func (s *vortexEventSender) updateLocalMap(entities []identityapi.RegisterEntityResponse) {
	s.localEntityMap.CleanOld()
	for _, e := range entities {
		s.localEntityMap.Put(e.Key, e.ID)
	}
}

// entityIDResolverWorker resolves remote entity IDs
// Batches several ID queries into 1 register request until a limit of size or timeout is reached.
// Is a background runner aimed to be launched within a goroutine.
func (s *vortexEventSender) entityIDResolverWorker(ctx context2.Context) {
	batchC := make(chan []eventVortexData)
	events := make([]eventVortexData, s.registerBatchSize)
	t := time.NewTicker(s.registerFrequency)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			s.flushRegister(events)
			return

		case ev := <-s.eventsWithoutID:
			events = append(events, ev)
			if len(events) >= s.registerBatchSize {
				select {
				case batchC <- events:
				case <-s.stopChannel:
					return
				}
			}

		case <-t.C:
			s.flushRegister(events)
			events = make([]eventVortexData, s.registerBatchSize)

		case events := <-batchC:
			s.flushRegister(events)
			events = make([]eventVortexData, s.registerBatchSize)
		}
	}
}

func (s *vortexEventSender) flushRegister(events []eventVortexData) {
	entities := make([]identityapi.RegisterEntity, s.registerBatchSize)
	for _, ev := range events {
		// events is a sized array of empty elements, not a slice
		if ev.entityKey == "" {
			return
		}

		entities = append(entities, identityapi.NewRegisterEntity(ev.entityKey))
	}

	// launches http request
	idsRes, err := s.provideIDs(s.agentIDProvide(), entities)
	if err != nil {
		logRegisterErr(entities, err)
	}
	if len(idsRes) < 1 {
		logEmptyRegisterErr(entities)
		return
	}

	for _, ev := range events {
		found := false
		for _, idRes := range idsRes {
			if idRes.Key == ev.entityKey {
				ev.entityID = idRes.ID
				s.updateLocalMap(idsRes)
				select {
				case s.eventsWithID <- ev:
				case <-s.stopChannel:
					return
				}
				found = true
				break
			}
		}
		if !found {
			vlog.WithField("entityKey", ev.entityKey).Error("event id not available on register response")
		}
	}
}

// Collect events from the queue and accumulate them into batches which can be sent up to metrics ingest.
// This hands off each batch to another channel which is consumed by a routine which does the actual post.
// We send batches on a timer or on accumulating some number of events in the current batch.
//
// We have a two-channel approach (event -> eventQueue -> batch -> batchQueue -> HTTP post) to minimize
// the impact of high-latency HTTP calls. If HTTP calls are slow, we'll still be able to run the event
// queue receiver and accumulate a reasonable number of batches before we fill up on batches as well.
func (s *vortexEventSender) accumulateBatches() {
	var batch eventVortexBatch
	var batchBytes int // Accumulated batch size in bytes

	ctx, cancel := context2.WithCancel(context2.Background())

	// Spawn worker consumers for events without ID in the localEntityMap
	for w := 0; w < s.registerWorkers; w++ {
		go s.entityIDResolverWorker(ctx)
	}

	sendTimerD := EVENT_BATCH_TIMER_DURATION * time.Second
	sendTimer := time.NewTimer(sendTimerD)
	for {
		select {
		case <-s.stopChannel:
			// Stop channel has been closed - exit.
			// There might still be some events in the queue, but they'll still be there in case we start the sender back up.
			cancel()
			return

		case event := <-s.eventQueue:
			if event.IsAgent() {
				event.entityID = s.agentIDProvide().ID

				select {
				case s.eventsWithID <- event:
				case <-s.stopChannel:
					return
				}
			} else {
				entityID, found := s.localEntityMap.Get(event.entityKey)
				if found {
					event.entityID = entityID
					select {
					case s.eventsWithID <- event:
					case <-s.stopChannel:
						return
					}
					continue
				}
				select {
				case s.eventsWithoutID <- event:
				case <-s.stopChannel:
					return
				}
			}

		case event := <-s.eventsWithID:
			if batchBytes+len(event.data) > s.maxMetricsBatchSizeBytes || len(batch) == MAX_EVENT_BATCH_COUNT {
				// Current batch + this event would either be too many events or too many bytes, so queue the batch first.
				select {
				case s.batchQueue <- batch:
					batch = make(eventVortexBatch, 0)
					batchBytes = 0
				case <-s.stopChannel:
					return
				}
			}
			batch = append(batch, event)
			batchBytes += len(event.data)

		case <-sendTimer.C:
			// Timer has fired - send any queued events to ensure a minimum delay in sending.
			if len(batch) > 0 {
				select {
				case s.batchQueue <- batch:
					batch = make(eventVortexBatch, 0)
					batchBytes = 0
				case <-s.stopChannel:
					return
				}
			}
			sendTimer.Reset(sendTimerD)
		}
	}
}

func logRegisterErr(entities []identityapi.RegisterEntity, err error) {
	keys := []string{}
	for _, e := range entities {
		keys = append(keys, e.Key.String())
	}
	vlog.WithError(err).WithField("keys", keys).Error("problems registering entities")
}

func logEmptyRegisterErr(entities []identityapi.RegisterEntity) {
	keys := []string{}
	for _, e := range entities {
		keys = append(keys, e.Key.String())
	}
	vlog.WithField("keys", keys).Error("empty ids registering entities")
}

func newMetricVortexPost(entityKey entity.Key, entityID, agentID entity.ID) *MetricVortexPost {
	return &MetricVortexPost{
		EntityKey:        entityKey,
		EntityID:         entityID,
		IsAgent:          entityID == agentID,
		ReportingAgentID: agentID,
	}
}

// Wait for queued batches and send any to the ingest API
func (s *vortexEventSender) sendBatches() {
	retryBO := backoff.NewDefaultBackoff()
	for {
		select {

		case batch := <-s.batchQueue:
			agentKey := ""
			dataByEntity := make(map[entity.Key]*MetricVortexPost)
			// We need to rebuild the array of events as a []json.RawMessage, or else JSON marshalling won't handle them correctly.
			agentID := s.agentIDProvide()
			for _, event := range batch {
				entityData := dataByEntity[event.entityKey]
				if entityData == nil {
					entityData = newMetricVortexPost(event.entityKey, event.entityID, agentID.ID)
					dataByEntity[event.entityKey] = entityData
				}
				entityData.Events = append(entityData.Events, event.data)
				if event.agentKey != "" {
					agentKey = event.agentKey
				}
			}

			var bulkPost MetricVortexPostBatch
			for _, entityData := range dataByEntity {
				vlog.WithFields(logrus.Fields{
					"key":          entityData.EntityKey,
					"eventsNumber": len(entityData.Events),
				}).Debug("Sending events to metrics-ingest.")

				bulkPost = append(bulkPost, entityData)
			}

			err := s.doPost(bulkPost, agentKey)

			if err == nil {
				atomic.StoreUint32(s.sendErrorCount, 0)
				retryBO.Reset()
				continue
			}

			currentSendErrCount := atomic.AddUint32(s.sendErrorCount, 1)
			vlog.WithError(err).WithField("sendErrorCount", currentSendErrCount).Error("metric sender can't process")

			e, ok := err.(*errRetry)
			if !ok {
				continue
			}

			if e.retryPolicy.After > 0 {
				vlog.WithField("retryAfter", e.retryPolicy.After).Debug("Metric sender retry requested.")
				retryBO.Reset()
				s.backoff(e.retryPolicy.After)
				continue
			}
			retryBOAfter := retryBO.DurationWithMax(e.retryPolicy.MaxBackOff)
			vlog.WithField("retryBackoffAfter", retryBOAfter).Debug("Metric sender backoff and retry requested.")
			s.backoff(retryBOAfter)

		case <-s.stopChannel:
			// Stop channel has been closed - exit.
			// There might still be some batches in the queue, but they'll still be there in case we start the sender back up.
			return
		}
	}
}

// backoff waits for the specified duration or a signal from the stop
// channel, whichever happens first.
func (s *vortexEventSender) backoff(d time.Duration) {
	backoffTimer := s.getBackoffTimer(d)
	select {
	case <-s.stopChannel:
	case <-backoffTimer.C:
	}
}

// Make one HTTP call to push a load of events up to the server
func (s *vortexEventSender) doPost(post []*MetricVortexPost, agentKey string) error {

	if agentKey == "" {
		vlog.Warn("no available agent-key on metrics sender")
	}

	agentID := s.Context.AgentID()

	if agentID.IsEmpty() {
		// should never happen, nor send data to backend
		return fmt.Errorf("empty agent-id on metrics sender")
	}

	postBytes, err := json.Marshal(post)
	if err != nil {
		return fmt.Errorf("Could not marshal events object [%v]: %v", post, err)
	}

	// GZIP
	var reqBuf *bytes.Buffer
	if s.Context.Config().PayloadCompressionLevel > gzip.NoCompression {
		// GZIP
		reqBuf = &bytes.Buffer{}
		compressionLevel := s.Context.Config().PayloadCompressionLevel
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
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/events/bulk", s.metricIngestURL), reqBuf)
	if err != nil {
		return fmt.Errorf("Error creating event POST: %v", err)
	}

	if s.Context.Config().PayloadCompressionLevel > gzip.NoCompression {
		req.Header.Set("Content-Encoding", "gzip")
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set(backendhttp.LicenseHeader, s.licenseKey)
	req.Header.Set(backendhttp.EntityKeyHeader, agentKey)
	req.Header.Set(backendhttp.AgentEntityIdHeader, agentID.String())

	resp, err := s.HttpClient(req)
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
			vlog.WithError(err).Debug(
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
