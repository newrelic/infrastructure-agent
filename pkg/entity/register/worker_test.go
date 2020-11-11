package register

import (
	"context"
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/backend/backoff"
	"github.com/stretchr/testify/assert"
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
	}
}

const (
	agentVersion = "testVersion"
)

type fakeClient struct {
	ids []entity.ID
	err error
}

var registerCallsCount = 0

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
	registerCallsCount++
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
	reqsToRegisterQueue := make(chan fwrequest.EntityFwRequest, 1)
	reqsRegisteredQueue := make(chan fwrequest.EntityFwRequest, 1)

	agentIdentity := func() entity.Identity {
		return entity.Identity{ID: 13}
	}

	ids := []entity.ID{123, 456}

	config := WorkerConfig{
		MaxBatchSize:      2,
		MaxBatchSizeBytes: MB,
		MaxBatchDuration:  50 * time.Millisecond,
		MaxRetryBo:        0,
	}
	w := NewWorker(agentIdentity, newClientReturning(ids...), backoff.NewDefaultBackoff(), reqsToRegisterQueue, reqsRegisteredQueue, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go w.Run(ctx)

	reqsToRegisterQueue <- fwrequest.NewEntityFwRequest(protocol.Dataset{}, entity.EmptyID, fwrequest.FwRequestMeta{}, protocol.IntegrationMetadata{}, agentVersion)

	for registeredCount := 0; registeredCount < len(ids); registeredCount++ {
		select {
		case result := <-reqsRegisteredQueue:
			assert.Equal(t, ids[registeredCount], result.ID())
		case <-time.NewTimer(200 * time.Millisecond).C:
			t.Error("no register response")
		}
	}
}

func TestWorker_Run_SendsWhenMaxBatchBytesSizeIsReached(t *testing.T) {
	registerCallsCount = 0
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
	assert.Equal(t, 2, registerCallsCount)
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
