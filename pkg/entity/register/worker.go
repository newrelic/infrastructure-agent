package register

import (
	"context"
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/backend/backoff"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent/id"
	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/fwrequest"
)

var (
	wlog = log.WithComponent("RegisterWorker")
)

type worker struct {
	agentIDProvide      id.Provide
	client              identityapi.RegisterClient
	retryBo             *backoff.Backoff
	maxRetryBo          time.Duration
	reqsToRegisterQueue <-chan fwrequest.EntityFwRequest
	reqsRegisteredQueue chan<- fwrequest.EntityFwRequest
	maxBatchSize        int
	maxBatchDuration    time.Duration
}

func NewWorker(
	agentIDProvide id.Provide,
	client identityapi.RegisterClient,
	retryBo *backoff.Backoff,
	maxRetryBo time.Duration,
	reqsToRegisterQueue <-chan fwrequest.EntityFwRequest,
	reqsRegisteredQueue chan<- fwrequest.EntityFwRequest,
	maxBatchSize int,
	maxBatchDuration time.Duration,
) *worker {
	return &worker{
		agentIDProvide:      agentIDProvide,
		client:              client,
		retryBo:             retryBo,
		maxRetryBo:          maxRetryBo,
		reqsToRegisterQueue: reqsToRegisterQueue,
		reqsRegisteredQueue: reqsRegisteredQueue,
		maxBatchSize:        maxBatchSize,
		maxBatchDuration:    maxBatchDuration,
	}
}

func (w *worker) Run(ctx context.Context) {
	timer := time.NewTimer(w.maxBatchDuration)

	// data for register batch call
	batch := make(map[entity.Key]fwrequest.EntityFwRequest, w.maxBatchSize)
	batchSize := 0

	for {
		select {
		case <-ctx.Done():
			return

		case req := <-w.reqsToRegisterQueue:
			batchSize++

			// TODO update when entity key retrieval is fixed
			eKey := entity.Key(req.Data.Entity.Name)
			batch[eKey] = req

			// TODO add 1MB payload size platform limitation

			if batchSize == w.maxBatchSize {
				timer.Reset(w.maxBatchDuration)
				w.send(ctx, batch, &batchSize)
			}

		case <-timer.C:
			if len(batch) > 0 {
				w.send(ctx, batch, &batchSize)
			}
			timer.Reset(w.maxBatchDuration)
		}
	}
}

func (w *worker) send(ctx context.Context, batch map[entity.Key]fwrequest.EntityFwRequest, batchSize *int) {
	defer w.resetBatch(batch, batchSize)

	var entities []entity.Fields
	for _, r := range batch {
		entities = append(entities, r.Data.Entity)
	}

	responses := w.registerEntitiesWithRetry(ctx, entities)

	for _, resp := range responses {
		if resp.ErrorMsg != "" {
			wlog.WithError(fmt.Errorf(resp.ErrorMsg)).
				WithField("entityName", resp.Name).
				Errorf("failed to register entity")
			continue
		}

		if len(resp.Warnings) > 0 {
			for _, warn := range resp.Warnings {
				wlog.
					WithField("entityName", resp.Name).
					WithField("entityID", resp.ID).
					Errorf("entity registered with warnings: %s", warn)
			}
		}

		r, ok := batch[entity.Key(resp.Name)]
		if !ok {
			wlog.
				WithField("entityName", resp.Name).
				WithField("entityID", resp.ID).
				WithField("entityName", resp.Name).
				Errorf("entityName returned by register not found in the entities batch")
			continue
		} else {
			r.RegisteredWith(resp.ID)
			w.reqsRegisteredQueue <- r
		}
	}
}

// registerEntitiesWithRetry will submit entities to the backend for registration.
// In case of StatusCodeConFailure or StatusCodeLimitExceed errors it will retry with backoff.
// for other errors, data will be discarded.
func (w *worker) registerEntitiesWithRetry(ctx context.Context, entities []entity.Fields) []identityapi.RegisterEntityResponse {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		var err error
		responses, err := w.client.RegisterBatchEntities(w.agentIDProvide().ID, entities)
		if err == nil {
			w.retryBo.Reset()
			return responses
		}

		e, ok := err.(*identityapi.RegisterEntityError)
		if ok {
			if e.ShouldRetry() {
				retryBOAfter := w.retryBo.DurationWithMax(w.maxRetryBo)
				wlog.WithField("retryBackoffAfter", retryBOAfter).Debug("register request retry backoff.")
				w.retryBo.Backoff(ctx, retryBOAfter)
				continue
			}
		}
		wlog.WithError(err).
			Error("entity register request error, discarding entities.")
		break
	}
	return nil
}

func (w *worker) resetBatch(batch map[entity.Key]fwrequest.EntityFwRequest, batchSize *int) {
	*batchSize = 0
	for key := range batch {
		delete(batch, key)
	}
}
