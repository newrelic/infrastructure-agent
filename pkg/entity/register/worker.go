package register

import (
	"context"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent/id"
	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/fwrequest"
)

type worker struct {
	agentIDProvide      id.Provide
	client              identityapi.RegisterClient
	reqsToRegisterQueue <-chan fwrequest.EntityFwRequest
	reqsRegisteredQueue chan<- fwrequest.EntityFwRequest
	maxBatchSize        int
	maxBatchDuration    time.Duration
}

func NewWorker(
	agentIDProvide id.Provide,
	client identityapi.RegisterClient,
	reqsToRegisterQueue <-chan fwrequest.EntityFwRequest,
	reqsRegisteredQueue chan<- fwrequest.EntityFwRequest,
	maxBatchSize int,
	maxBatchDuration time.Duration,
) *worker {
	return &worker{
		agentIDProvide:      agentIDProvide,
		client:              client,
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
				w.send(batch, &batchSize)
			}

		case <-timer.C:
			if len(batch) > 0 {
				w.send(batch, &batchSize)
			}
			timer.Reset(w.maxBatchDuration)
		}
	}
}

func (w *worker) send(batch map[entity.Key]fwrequest.EntityFwRequest, batchSize *int) {
	defer w.resetBatch(batch, batchSize)

	var entities []entity.Fields
	for _, r := range batch {
		entities = append(entities, r.Data.Entity)
	}
	responses, _, errClient := w.client.RegisterBatchEntities(w.agentIDProvide().ID, entities)
	if errClient != nil {
		// TODO error handling
		return
	}

	for _, resp := range responses {
		if resp.Err != "" {
			// TODO error handling
			continue
		}

		r, ok := batch[resp.Key]
		if !ok {
			// TODO error handling
		} else {
			r.RegisteredWith(resp.ID)
			w.reqsRegisteredQueue <- r
		}
	}
}

func (w *worker) resetBatch(batch map[entity.Key]fwrequest.EntityFwRequest, batchSize *int) {
	*batchSize = 0
	for key := range batch {
		delete(batch, key)
	}
}
