package register

import (
	"context"
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/backend/backoff"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/fwrequest"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
)

// Payload size.
const MB = 1000 * 1000

func newClientReturning(ids ...entity.ID) identityapi.RegisterClient {
	return &fakeClient{
		ids: ids,
		wg:  &sync.WaitGroup{},
	}
}

const (
	agentVersion = "testVersion"
)

type fakeClient struct {
	wg        *sync.WaitGroup
	mutex     sync.Mutex
	waitFor   int
	ids       []entity.ID
	collector [][]string
	err       error
}

func (c *fakeClient) RegisterBatchEntities(agentEntityID entity.ID, entities []entity.Fields) (r []identityapi.RegisterEntityResponse, err error) {
	if c.err != nil {
		return nil, c.err
	}
	r = []identityapi.RegisterEntityResponse{}
	for i, id := range c.ids {
		var name string
		if len(entities) > i {
			name = entities[i].Name
			r = append(r, identityapi.RegisterEntityResponse{Name: name, ID: id})
		}
	}

	names := make([]string, len(entities))
	for i, e := range entities {
		names[i] = e.Name
	}
	c.collector = append(c.collector, names)

	c.internalWaitGroupDone()
	return
}

func (c *fakeClient) RegisterEntity(agentEntityID entity.ID, entity entity.Fields) (r identityapi.RegisterEntityResponse, err error) {
	// won't be called
	return
}

func (c *fakeClient) RegisterEntitiesRemoveMe(agentEntityID entity.ID, entities []identityapi.RegisterEntity) (r []identityapi.RegisterEntityResponse, t time.Duration, err error) {
	// won't be called
	return
}

func (c *fakeClient) assertRecordedData(t *testing.T, names [][]string) {
	require.Equal(t, len(names), len(c.collector))

	for i, n := range names {
		assert.ElementsMatch(t, n, c.collector[i])
	}
}

func (c *fakeClient) internalWaitGroupAdd(times int) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.waitFor = times
	c.wg.Add(times)
}

func (c *fakeClient) internalWaitGroupDone() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.waitFor > 0 {
		c.wg.Done()
		c.waitFor--
	}
}

func (c *fakeClient) internalWaitGroupRelease() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for c.waitFor > 0 {
		c.internalWaitGroupDone()
	}
}

func (c *fakeClient) internalWaitGroupWait() {
	c.wg.Wait()
}

func TestWorker_Run_SendsWhenMaxTimeIsReached(t *testing.T) {
	reqsToRegisterQueue := make(chan fwrequest.EntityFwRequest, 1)
	reqsRegisteredQueue := make(chan fwrequest.EntityFwRequest, 1)

	agentIdentity := func() entity.Identity {
		return entity.Identity{ID: 13}
	}

	config := WorkerConfig{
		MaxBatchSize:      2,
		MaxBatchSizeBytes: MB,
		MaxBatchDuration:  50 * time.Millisecond,
		MaxRetryBo:        0,
	}
	w := NewWorker(agentIdentity, newClientReturning(123), backoff.NewDefaultBackoff(), reqsToRegisterQueue, reqsRegisteredQueue, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go w.Run(ctx)

	reqsToRegisterQueue <- fwrequest.NewEntityFwRequest(protocol.Dataset{}, entity.EmptyID, fwrequest.FwRequestMeta{}, protocol.IntegrationMetadata{}, agentVersion)

	select {
	case result := <-reqsRegisteredQueue:
		assert.Equal(t, "123", result.ID().String())
	case <-time.NewTimer(200 * time.Millisecond).C:
		t.Error("no register response")
	}
}

func TestWorker_Run_SendsWhenMaxBatchSizeIsReached(t *testing.T) {
	testCases := []struct {
		name               string
		batchSize          int
		batchesCount       int
		entitiesCount      int
		expectedEntityName [][]string //TODO fix naming
		batchDuration      time.Duration
		timeout            time.Duration
	}{
		{
			name:               "full batch",
			batchSize:          1,
			batchesCount:       1,
			entitiesCount:      1,
			expectedEntityName: [][]string{{"test-0"}},
			batchDuration:      50 * time.Millisecond,
			timeout:            100 * time.Millisecond,
		},
		{
			name:               "The Hitchhiker's Guide to Galaxy - 42",
			batchSize:          2,
			batchesCount:       2,
			entitiesCount:      4,
			expectedEntityName: [][]string{{"test-0", "test-1"}, {"test-2", "test-3"}},
			batchDuration:      50 * time.Millisecond,
			timeout:            100 * time.Millisecond,
		},
		{
			name:               "one full batch, one pending to be full",
			batchSize:          2,
			batchesCount:       1,
			entitiesCount:      3,
			expectedEntityName: [][]string{{"test-0", "test-1"}},
			batchDuration:      200 * time.Millisecond,
			timeout:            100 * time.Millisecond,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			reqsToRegisterQueue := make(chan fwrequest.EntityFwRequest, test.entitiesCount)

			agentIdentity := func() entity.Identity {
				return entity.Identity{ID: 13}
			}

			ids := []entity.ID{123, 456}

			config := WorkerConfig{
				MaxBatchSize:      test.batchSize,
				MaxBatchSizeBytes: MB,
				MaxBatchDuration:  test.batchDuration,
				MaxRetryBo:        0,
			}

			client := &fakeClient{
				ids:       ids,
				wg:        &sync.WaitGroup{},
				collector: make([][]string, 0),
			}
			client.internalWaitGroupAdd(test.batchesCount)

			w := NewWorker(agentIdentity, client, backoff.NewDefaultBackoff(), reqsToRegisterQueue, make(chan fwrequest.EntityFwRequest, test.entitiesCount), config)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			go w.Run(ctx)

			for i := 0; i < test.entitiesCount; i++ {
				reqsToRegisterQueue <- fwrequest.NewEntityFwRequest(protocol.Dataset{Entity: entity.Fields{
					Name: fmt.Sprintf("test-%d", i),
				}}, entity.EmptyID, fwrequest.FwRequestMeta{}, protocol.IntegrationMetadata{}, agentVersion)
			}

			client.internalWaitGroupWait()
			client.assertRecordedData(t, test.expectedEntityName)
		})
	}
}

func TestWorker_Run_SendsWhenMaxBatchBytesSizeIsReached(t *testing.T) {
	reqsToRegisterQueue := make(chan fwrequest.EntityFwRequest, 10)
	reqsRegisteredQueue := make(chan fwrequest.EntityFwRequest, 10)
	agentIdentity := func() entity.Identity {
		return entity.Identity{ID: 13}
	}
	ids := []entity.ID{123, 456}
	entityFields1 := entity.Fields{
		Name: "test1",
	}
	entityFields2 := entity.Fields{
		Name: "test2",
	}
	// Given a MaxBatchSize(number of elements of 100), we send even 1 if the maxBytesSize is reached.
	config := WorkerConfig{
		MaxBatchSize:      1000,
		MaxBatchSizeBytes: entityFields1.JsonSize(),
		MaxBatchDuration:  50 * time.Millisecond,
		MaxRetryBo:        0,
	}
	fakeRegisterClient := newClientReturning(ids...)
	w := NewWorker(
		agentIdentity,
		fakeRegisterClient,
		backoff.NewDefaultBackoff(),
		reqsToRegisterQueue,
		reqsRegisteredQueue,
		config,
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)
	reqsToRegisterQueue <- fwrequest.NewEntityFwRequest(protocol.Dataset{Entity: entityFields1}, entity.EmptyID, fwrequest.FwRequestMeta{}, protocol.IntegrationMetadata{}, agentVersion)
	// Second request will cause the batch split.
	reqsToRegisterQueue <- fwrequest.NewEntityFwRequest(protocol.Dataset{Entity: entityFields2}, entity.EmptyID, fwrequest.FwRequestMeta{}, protocol.IntegrationMetadata{}, agentVersion)
	timeout := 200 * time.Millisecond
	errorMessage := "timeout reached, no messages returned"
	select {
	case <-reqsRegisteredQueue:
	case <-time.After(timeout):
		t.Error(errorMessage)
	}
	select {
	case <-reqsRegisteredQueue:
	case <-time.After(timeout):
		t.Error(errorMessage)
	}
	//assert.Equal(t, 2, registerCallsCount)
}

func TestWorker_registerEntitiesWithRetry_OnError_RetryBackoff(t *testing.T) {
	reqsToRegisterQueue := make(chan fwrequest.EntityFwRequest, 0)
	reqsRegisteredQueue := make(chan fwrequest.EntityFwRequest, 0)

	agentIdentity := func() entity.Identity {
		return entity.Identity{ID: 13}
	}

	client := &fakeClient{
		// StatusCodeLimitExceed is an e.g. of error for which we should retry with backoff.
		err: identityapi.NewRegisterEntityError("err", identityapi.StatusCodeLimitExceed, fmt.Errorf("err")),
	}

	backoff := backoff.NewDefaultBackoff()
	backoffCh := make(chan time.Duration)
	backoff.GetBackoffTimer = func(d time.Duration) *time.Timer {
		select {
		case backoffCh <- d:
		default:
		}
		return time.NewTimer(0)
	}

	config := WorkerConfig{
		MaxBatchSize:      1,
		MaxBatchSizeBytes: MB,
		MaxBatchDuration:  50 * time.Millisecond,
		MaxRetryBo:        0,
	}
	w := NewWorker(agentIdentity, client, backoff, reqsToRegisterQueue, reqsRegisteredQueue, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result := make(chan []identityapi.RegisterEntityResponse, 1)
	go func() {
		result <- w.registerEntitiesWithRetry(ctx, []entity.Fields{{Name: "test"}})
	}()

	select {
	case <-backoffCh: // Success
	case <-result:
		t.Error("registerEntitiesWithRetry should retry")
	case <-time.NewTimer(200 * time.Millisecond).C:
		t.Error("Backoff not called")
	}

	cancel()
	select {
	case actual := <-result:
		var expected []identityapi.RegisterEntityResponse = nil
		assert.Equal(t, expected, actual)
	case <-time.NewTimer(200 * time.Millisecond).C:
		t.Error("registerEntitiesWithRetry should stop")
	}
}

func TestWorker_registerEntitiesWithRetry_OnError_Discard(t *testing.T) {
	reqsToRegisterQueue := make(chan fwrequest.EntityFwRequest, 0)
	reqsRegisteredQueue := make(chan fwrequest.EntityFwRequest, 0)

	agentIdentity := func() entity.Identity {
		return entity.Identity{ID: 13}
	}

	client := &fakeClient{
		// 400 is an e.g. of an error for which we should discard data.
		err: identityapi.NewRegisterEntityError("err", 400, fmt.Errorf("err")),
	}

	backoff := backoff.NewDefaultBackoff()
	backoffCh := make(chan time.Duration)
	backoff.GetBackoffTimer = func(d time.Duration) *time.Timer {
		select {
		case backoffCh <- d:
		default:
		}
		return time.NewTimer(0)
	}

	config := WorkerConfig{
		MaxBatchSize:      1,
		MaxBatchSizeBytes: MB,
		MaxBatchDuration:  50 * time.Millisecond,
		MaxRetryBo:        0,
	}
	w := NewWorker(agentIdentity, client, backoff, reqsToRegisterQueue, reqsRegisteredQueue, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	response := make(chan []identityapi.RegisterEntityResponse, 1)
	go func() {
		response <- w.registerEntitiesWithRetry(ctx, []entity.Fields{{Name: "test"}})
	}()

	select {
	case actual := <-response:
		var expected []identityapi.RegisterEntityResponse = nil
		assert.Equal(t, expected, actual)
	case <-backoffCh:
		t.Error("backoff should not be called")
	case <-time.NewTimer(200 * time.Millisecond).C:
		t.Error("registerEntitiesWithRetry should stop")
	}
}

func TestWorker_registerEntitiesWithRetry_Success(t *testing.T) {
	reqsToRegisterQueue := make(chan fwrequest.EntityFwRequest, 0)
	reqsRegisteredQueue := make(chan fwrequest.EntityFwRequest, 0)

	agentIdentity := func() entity.Identity {
		return entity.Identity{ID: 13}
	}

	client := &fakeClient{
		ids: []entity.ID{13},
		// no err from backend.
		err: nil,
	}

	backoff := backoff.NewDefaultBackoff()
	backoffCh := make(chan time.Duration)
	backoff.GetBackoffTimer = func(d time.Duration) *time.Timer {
		select {
		case backoffCh <- d:
		default:
		}
		return time.NewTimer(0)
	}

	config := WorkerConfig{
		MaxBatchSize:      1,
		MaxBatchSizeBytes: MB,
		MaxBatchDuration:  50 * time.Millisecond,
		MaxRetryBo:        0,
	}
	w := NewWorker(agentIdentity, client, backoff, reqsToRegisterQueue, reqsRegisteredQueue, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	response := make(chan []identityapi.RegisterEntityResponse, 1)
	go func() {
		response <- w.registerEntitiesWithRetry(ctx, []entity.Fields{{Name: "test"}})
	}()
	select {
	case actual := <-response:
		assert.Equal(t, 1, len(actual))
		assert.Equal(t, "13", actual[0].ID.String())
	case <-backoffCh:
		t.Error("backoff should not be called")
	case <-time.NewTimer(200 * time.Millisecond).C:
		t.Error("registerEntitiesWithRetry should stop")
	}
}
