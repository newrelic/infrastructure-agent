// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package register

import (
	"context"
	"fmt"
	"io/ioutil"
	"sync"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/backend/backoff"
	log "github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	sync.RWMutex
	collector [][]string
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
			// Simulate an entity that generates error
			if name == "error" {
				r = append(r, identityapi.RegisterEntityResponse{Name: name, ErrorMsg: "Invalid entityName"})
			} else if name == "warnings" {
				r = append(r, identityapi.RegisterEntityResponse{
					Name: name,
					Warnings: []string{
						"Too many metadata, dropping ...",
						"Invalid metadata: ...",
					},
				})
			} else {
				r = append(r, identityapi.RegisterEntityResponse{Name: name, ID: id})
			}
		}
	}

	names := make([]string, len(entities))
	for i, e := range entities {
		names[i] = e.Name
	}
	c.Lock()
	c.collector = append(c.collector, names)
	c.Unlock()
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
	c.RLock()
	defer c.RUnlock()

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
		expectedEntityName   [][]string
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
			timeout:              200 * time.Millisecond,
		},
		{
			name:                 "given_a_batch_byte_limit_of_2_entities_size_when_2_entities_are_submitted_then_register_is_called_in_a_single_batch",
			maxBatchSize:         10,                                              // high number to prevent send by this setting
			maxBatchByteSize:     (&entity.Fields{Name: "test-1"}).JsonSize() * 2, // calc based on number of entities per batch
			givenEntitiesCount:   2,
			expectedBatchesCount: 1,
			expectedEntityName:   [][]string{{"test-0", "test-1"}},
			batchDuration:        time.Second,
			timeout:              100 * time.Millisecond,
		},
		{
			name:                 "given_a_batch_byte_limit_of_2_entities_size_when_3_entities_are_submitted_then_register_is_called_with_1_batch_by_limit_and_other_batch_by_timer",
			maxBatchSize:         10,                                              // high number to prevent send by this setting
			maxBatchByteSize:     (&entity.Fields{Name: "test-1"}).JsonSize() * 2, // calc based on number of entities per batch
			givenEntitiesCount:   3,
			expectedBatchesCount: 1,
			expectedEntityName:   [][]string{{"test-0", "test-1"}, {"test-2"}},
			batchDuration:        50 * time.Millisecond,
			timeout:              250 * time.Millisecond,
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
	worker := NewWorker(agentIdentity, client, backoff, reqsToRegisterQueue, reqsRegisteredQueue, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result := make(chan []identityapi.RegisterEntityResponse, 1)
	go func() {
		result <- worker.registerEntitiesWithRetry(ctx, []entity.Fields{{Name: "test"}})
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
	worker := NewWorker(agentIdentity, client, backoff, reqsToRegisterQueue, reqsRegisteredQueue, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	response := make(chan []identityapi.RegisterEntityResponse, 1)
	go func() {
		response <- worker.registerEntitiesWithRetry(ctx, []entity.Fields{{Name: "test"}})
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
	worker := NewWorker(agentIdentity, client, backoff, reqsToRegisterQueue, reqsRegisteredQueue, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	response := make(chan []identityapi.RegisterEntityResponse, 1)
	go func() {
		response <- worker.registerEntitiesWithRetry(ctx, []entity.Fields{{Name: "test"}})
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

func TestWorker_send_Logging_VerboseEnabled(t *testing.T) {
	expectedErrs := []string{
		"Invalid entityName",
	}
	expectedWarnings := []string{
		"Too many metadata, dropping ...",
		"Invalid metadata: ...",
	}

	// When the request is successful but some entities fail, we log only when verbose is enabled.
	reqsToRegisterQueue := make(chan fwrequest.EntityFwRequest, 0)
	reqsRegisteredQueue := make(chan fwrequest.EntityFwRequest, 0)

	agentIdentity := func() entity.Identity {
		return entity.Identity{ID: 12}
	}

	client := &fakeClient{
		ids: []entity.ID{13, 14},
		// no err from backend.
		err: nil,
	}

	config := WorkerConfig{
		VerboseLogLevel: 1,
	}
	worker := NewWorker(agentIdentity, client, backoff.NewDefaultBackoff(), reqsToRegisterQueue, reqsRegisteredQueue, config)

	batch := map[entity.Key]fwrequest.EntityFwRequest{
		entity.Key("error"): {
			Data: protocol.Dataset{
				Entity: entity.Fields{Name: "error"},
			},
		},
		entity.Key("warnings"): {
			Data: protocol.Dataset{
				Entity: entity.Fields{Name: "warnings"},
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-reqsRegisteredQueue:
				// empty the registered queue to process new requests.
			case <-time.After(5 * time.Second):
				cancel()
				t.Error("Timeout while executing the test")
			}
		}
	}()

	hook := new(test.Hook)
	log.AddHook(hook)
	log.SetOutput(ioutil.Discard)

	batchSizeBytes := 10000
	worker.send(ctx, batch, &batchSizeBytes)

	searchLogEntries := func(expectedMessages []string, level log.Level) (found bool) {
		for _, expectedMsg := range expectedMessages {
			for i, entry := range hook.AllEntries() {
				if entry.Level != level {
					continue
				}
				if val, ok := hook.AllEntries()[i].Data["error"]; ok {
					errStr := val.(error).Error()
					if errStr == expectedMsg {
						found = true
					}
				}
			}
		}
		return
	}

	assert.Eventually(t, func() bool {
		ok := searchLogEntries(expectedErrs, log.ErrorLevel)
		return ok
	}, time.Second, 10*time.Millisecond,
		"expected to find error messages: %s", expectedErrs)

	assert.Eventually(t, func() bool {
		ok := searchLogEntries(expectedWarnings, log.WarnLevel)
		return ok
	}, time.Second, 10*time.Millisecond,
		"expected to find warning messages: %s \n", expectedWarnings)
}

func TestWorker_send_Logging_VerboseDisabled(t *testing.T) {
	// When the request is successful but some entities fail, we log only when verbose is enabled.
	reqsToRegisterQueue := make(chan fwrequest.EntityFwRequest, 0)
	reqsRegisteredQueue := make(chan fwrequest.EntityFwRequest, 0)

	agentIdentity := func() entity.Identity {
		return entity.Identity{ID: 12}
	}

	client := &fakeClient{
		ids: []entity.ID{13, 14},
		// no err from backend.
		err: nil,
	}

	config := WorkerConfig{
		VerboseLogLevel: 0,
	}
	worker := NewWorker(agentIdentity, client, backoff.NewDefaultBackoff(), reqsToRegisterQueue, reqsRegisteredQueue, config)

	batch := map[entity.Key]fwrequest.EntityFwRequest{
		entity.Key("error"): {
			Data: protocol.Dataset{
				Entity: entity.Fields{Name: "error"},
			},
		},
		entity.Key("warnings"): {
			Data: protocol.Dataset{
				Entity: entity.Fields{Name: "warnings"},
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-reqsRegisteredQueue:
				// empty the registered queue to process new requests.
			case <-time.After(5 * time.Second):
				cancel()
				t.Error("Timeout while executing the test")
			}
		}
	}()

	hook := new(test.Hook)
	log.AddHook(hook)
	log.SetOutput(ioutil.Discard)

	batchSizeBytes := 10000
	worker.send(ctx, batch, &batchSizeBytes)

	assert.Empty(t, hook.AllEntries())
}
func TestUpdateEntityMetadata(t *testing.T) {
	t.Parallel()
	expected := &entity.Fields{
		Name:         "WIN_SERVICE:testWindows:newrelic-infra",
		Type:         "WIN_SERVICE",
		IDAttributes: nil,
		DisplayName:  "New Relic Infrastructure Agent",
		Metadata: map[string]interface{}{
			"environment": "dev",
			"backup":      "true",
		},
	}
	labels := map[string]string{
		"environment": "dev",
		"backup":      "true",
	}
	entity := &entity.Fields{
		Name:         "WIN_SERVICE:testWindows:newrelic-infra",
		Type:         "WIN_SERVICE",
		IDAttributes: nil,
		DisplayName:  "New Relic Infrastructure Agent",
		Metadata:     map[string]interface{}{},
	}
	updateEntityMetadata(entity, labels)
	assert.Equal(t, expected, entity)
}
