package register

import (
	"context"
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/backend/backoff"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestWorker_Run_SendsWhenBatchLimitIsReached(t *testing.T) {
	testCases := []struct {
		name                 string
		maxBatchSize         int
		maxBatchByteSize     int
		expectedBatchesCount int
		givenEntitiesCount   int
		expectedEntityName   [][]string //TODO fix naming
		batchDuration        time.Duration
		timeout              time.Duration
	}{
		{
			name:                 "given_a_batch_limits_of_5_entities_when_5_entities_are_submitted_then_register_is_called_in_a_single_batch",
			maxBatchSize:         5,
			maxBatchByteSize:     MB,
			givenEntitiesCount:   5,
			expectedBatchesCount: 1,
			expectedEntityName:   [][]string{{"test-0", "test-1", "test-2", "test-3", "test-4"}},
			batchDuration:        50 * time.Millisecond,
			timeout:              100 * time.Millisecond,
		},
		{
			name:                 "given_a_batch_limit_of_2_entities_when_4_entities_are_submitted_then_register_is_called_in_2_batches",
			maxBatchSize:         2,
			maxBatchByteSize:     MB,
			givenEntitiesCount:   4,
			expectedBatchesCount: 2,
			expectedEntityName:   [][]string{{"test-0", "test-1"}, {"test-2", "test-3"}},
			batchDuration:        50 * time.Millisecond,
			timeout:              100 * time.Millisecond,
		},
		{
			name:                 "given_a_batch_limit_of_2_entities_when_3_entities_are_submitted_then_register_is_called_with_1_batch_by_limit_and_other_batch_by_timer",
			maxBatchSize:         2,
			maxBatchByteSize:     MB,
			givenEntitiesCount:   3,
			expectedBatchesCount: 1,
			expectedEntityName:   [][]string{{"test-0", "test-1"}, {"test-2"}},
			batchDuration:        50 * time.Millisecond,
			timeout:              100 * time.Millisecond,
		},
		{
			name:                 "given_a_batch_byte_limit_of_2_entities_size_when_2_entities_are_submitted_then_register_is_called_in_a_single_batch",
			maxBatchSize:         10,                                              // high number to prevent send by this setting
			maxBatchByteSize:     (&entity.Fields{Name: "test-1"}).JsonSize() * 2, //calc based on number of entities per batch
			givenEntitiesCount:   2,
			expectedBatchesCount: 1,
			expectedEntityName:   [][]string{{"test-0", "test-1"}},
			batchDuration:        time.Second,
			timeout:              100 * time.Millisecond,
		},
		{
			name:                 "given_a_batch_byte_limit_of_2_entities_size_when_3_entities_are_submitted_then_register_is_called_with_1_batch_by_limit_and_other_batch_by_timer",
			maxBatchSize:         10,                                              // high number to prevent send by this setting
			maxBatchByteSize:     (&entity.Fields{Name: "test-1"}).JsonSize() * 2, //calc based on number of entities per batch
			givenEntitiesCount:   3,
			expectedBatchesCount: 1,
			expectedEntityName:   [][]string{{"test-0", "test-1"}, {"test-2"}},
			batchDuration:        50 * time.Millisecond,
			timeout:              100 * time.Millisecond,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			reqsToRegisterQueue := make(chan fwrequest.EntityFwRequest, test.givenEntitiesCount)
			registeredQueue := make(chan fwrequest.EntityFwRequest, test.givenEntitiesCount)

			agentIdentity := func() entity.Identity {
				return entity.Identity{ID: 13}
			}

			ids := make([]entity.ID, test.maxBatchSize)
			for i := 0; i < test.maxBatchSize; i++ {
				ids[i] = entity.ID(i)
			}

			config := WorkerConfig{
				MaxBatchSize:      test.maxBatchSize,
				MaxBatchSizeBytes: test.maxBatchByteSize,
				MaxBatchDuration:  test.batchDuration,
				MaxRetryBo:        0,
			}

			client := &fakeClient{
				ids:       ids,
				collector: make([][]string, 0),
			}

			w := NewWorker(agentIdentity, client, backoff.NewDefaultBackoff(), reqsToRegisterQueue, registeredQueue, config)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			go w.Run(ctx)

			for i := 0; i < test.givenEntitiesCount; i++ {
				reqsToRegisterQueue <- fwrequest.NewEntityFwRequest(protocol.Dataset{Entity: entity.Fields{
					Name: fmt.Sprintf("test-%d", i),
				}}, entity.EmptyID, fwrequest.FwRequestMeta{}, protocol.IntegrationMetadata{}, agentVersion)
			}

			// wait for the register to finish all the requests
			for i := 0; i < test.givenEntitiesCount; i++ {
				select {
				case <-registeredQueue:
				case <-time.After(test.timeout):
					assert.Fail(t, "timeout exceeded waiting for register")
				}
			}

			client.assertRecordedData(t, test.expectedEntityName)
		})
	}
}

func TestWorker_Run_EntityGraterThanMaxByteSize(t *testing.T) {
	reqsToRegisterQueue := make(chan fwrequest.EntityFwRequest, 0)
	registeredQueue := make(chan fwrequest.EntityFwRequest, 1)

	agentIdentity := func() entity.Identity {
		return entity.Identity{ID: 13}
	}
	config := WorkerConfig{
		MaxBatchSize:      2,
		MaxBatchSizeBytes: (&entity.Fields{Name: "test-1"}).JsonSize(),
		MaxBatchDuration:  50 * time.Millisecond,
		MaxRetryBo:        0,
	}

	client := &fakeClient{
		ids: []entity.ID{1},
	}

	w := NewWorker(agentIdentity, client, backoff.NewDefaultBackoff(), reqsToRegisterQueue, registeredQueue, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go w.Run(ctx)

	reqsToRegisterQueue <- fwrequest.NewEntityFwRequest(protocol.Dataset{Entity: entity.Fields{
		Name: "this-entity-should-not-pass",
	}}, entity.EmptyID, fwrequest.FwRequestMeta{}, protocol.IntegrationMetadata{}, agentVersion)

	reqsToRegisterQueue <- fwrequest.NewEntityFwRequest(protocol.Dataset{Entity: entity.Fields{
		Name: "test-1",
	}}, entity.EmptyID, fwrequest.FwRequestMeta{}, protocol.IntegrationMetadata{}, agentVersion)

	select {
	case req := <-registeredQueue:
		assert.Equal(t, "test-1", req.Data.Entity.Name)
	}
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
