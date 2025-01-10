// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package register

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/backend/backoff"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/sirupsen/logrus"

	"github.com/newrelic/infrastructure-agent/internal/agent/id"
	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/fwrequest"
)

var (
	wlog = log.WithComponent("RegisterWorker")
)

// WorkerConfig will provide all configuration parameters for a register worker.
type WorkerConfig struct {
	MaxBatchSize      int
	MaxBatchSizeBytes int
	MaxBatchDuration  time.Duration
	MaxRetryBo        time.Duration
	VerboseLogLevel   int
}

type worker struct {
	agentIDProvide      id.Provide
	client              identityapi.RegisterClient
	retryBo             *backoff.Backoff
	reqsToRegisterQueue <-chan fwrequest.EntityFwRequest
	reqsRegisteredQueue chan<- fwrequest.EntityFwRequest
	config              WorkerConfig
}

func NewWorker(
	agentIDProvide id.Provide,
	client identityapi.RegisterClient,
	retryBo *backoff.Backoff,
	reqsToRegisterQueue <-chan fwrequest.EntityFwRequest,
	reqsRegisteredQueue chan<- fwrequest.EntityFwRequest,
	config WorkerConfig,
) *worker {
	return &worker{
		agentIDProvide:      agentIDProvide,
		client:              client,
		retryBo:             retryBo,
		reqsToRegisterQueue: reqsToRegisterQueue,
		reqsRegisteredQueue: reqsRegisteredQueue,
		config:              config,
	}
}

func (w *worker) Run(ctx context.Context) {
	timer := time.NewTimer(w.config.MaxBatchDuration)

	// data for register batch call
	batch := make(map[entity.Key]fwrequest.EntityFwRequest, w.config.MaxBatchSize)
	batchSizeBytes := 0
	for {
		select {
		case <-ctx.Done():
			return

		case req := <-w.reqsToRegisterQueue:
			entitySizeBytes := req.Data.Entity.JsonSize()

			// Drop entities that exceed the size limit
			if entitySizeBytes > w.config.MaxBatchSizeBytes {
				wlog.WithFields(logrus.Fields{
					"entity-size":       entitySizeBytes,
					"entity-name":       req.Data.Entity.Name,
					"maxBatchSizeBytes": w.config.MaxBatchSizeBytes,
				}).Errorf("cannot process entity because size exceeded")
				continue
			}

			// Check if requested entity will overpass the max bytes limits
			if batchSizeBytes+entitySizeBytes > w.config.MaxBatchSizeBytes {
				timer.Reset(w.config.MaxBatchDuration)
				w.send(ctx, batch, &batchSizeBytes)
			}
			// TODO update when entity key retrieval is fixed
			eKey := entity.Key(req.Data.Entity.Name)
			batch[eKey] = req
			batchSizeBytes += entitySizeBytes

			// Send if batch is full
			if batchSizeBytes == w.config.MaxBatchSizeBytes || len(batch) == w.config.MaxBatchSize {
				timer.Reset(w.config.MaxBatchDuration)
				w.send(ctx, batch, &batchSizeBytes)
			}
		case <-timer.C:
			if len(batch) > 0 {
				w.send(ctx, batch, &batchSizeBytes)
			}
			timer.Reset(w.config.MaxBatchDuration)
		}
	}
}

func (w *worker) send(ctx context.Context, batch map[entity.Key]fwrequest.EntityFwRequest, batchSizeBytes *int) {
	defer w.resetBatch(batch, batchSizeBytes)

	var entities []entity.Fields
	for _, r := range batch {
		entity := r.Data.Entity
		// Add labels to Metadata
		if r.Definition.Labels != nil && len(r.Definition.Labels) > 0 {
			for key, value := range r.Definition.Labels {
				entity.Metadata[key] = value
			}
		}
		entities = append(entities, entity)
	}

	responses := w.registerEntitiesWithRetry(ctx, entities)

	for _, resp := range responses {
		if resp.ErrorMsg != "" {
			if w.config.VerboseLogLevel > 0 {
				wlog.WithError(fmt.Errorf(resp.ErrorMsg)).
					WithField("entityName", resp.Name).
					Errorf("failed to register entity")
			}

			continue
		}

		if w.config.VerboseLogLevel > 0 && len(resp.Warnings) > 0 {
			for _, warn := range resp.Warnings {
				wlog.WithError(fmt.Errorf(warn)).
					WithField("entityName", resp.Name).
					WithField("entityID", resp.ID).
					Warn("entity registered with warnings")
			}
		}

		r, ok := batch[entity.Key(resp.Name)]
		if !ok {
			if w.config.VerboseLogLevel > 0 {
				wlog.
					WithField("entityName", resp.Name).
					WithField("entityID", resp.ID).
					WithField("entityName", resp.Name).
					Errorf("entityName returned by register not found in the entities batch")
			}
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

		// Backoff object it's shared between workers. If another worker is in backoff,
		// the current will also backoff.
		attempt := w.retryBo.Attempt()
		if attempt > 0 {
			retryBOAfter := w.retryBo.ForAttemptWithMax(attempt, w.config.MaxRetryBo)
			wlog.WithField("retryBackoffAfter", retryBOAfter).Debug("register request retry backoff.")
			w.retryBo.Backoff(ctx, retryBOAfter)
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
				w.retryBo.IncreaseAttempt()
				continue
			}
		}

		elog := wlog.WithError(err)
		ent, err := json.Marshal(entities)

		if err != nil {
			wlog.WithError(err).Error("cannot marshal entities to register")
		} else {
			elog = elog.WithTraceField("entities", string(ent))
		}

		elog.Error("entity register request error, discarding entities.")

		break
	}
	return nil
}

func (w *worker) resetBatch(batch map[entity.Key]fwrequest.EntityFwRequest, batchSizeBytes *int) {
	*batchSizeBytes = 0
	for key := range batch {
		delete(batch, key)
	}
}
