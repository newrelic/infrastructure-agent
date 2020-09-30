package register

import (
	"context"
	"sync"

	"github.com/newrelic/infrastructure-agent/internal/agent/id"
	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/fwrequest"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
)

type RegisteredEntitiesNameToID map[string]entity.ID
type UnregisteredEntityListWithWait struct {
	entities  UnregisteredEntityList
	waitGroup *sync.WaitGroup
}

type Reason string

type UnregisteredEntity struct {
	Reason Reason
	Err    error
	Entity protocol.Entity
}

type UnregisteredEntityList []UnregisteredEntity

func RunEntityIDResolverWorker(
	ctx context.Context,
	agentIDProvide id.Provide,
	client identityapi.RegisterClient,
	reqsToRegisterQueue <-chan fwrequest.EntityFwRequest,
	reqsRegisteredQueue chan<- fwrequest.EntityFwRequest,
	registerBatchSize int,
) {
	// data for register batch call
	batch := make(map[entity.Key]fwrequest.EntityFwRequest, registerBatchSize)
	batchSize := 0

	select {
	case <-ctx.Done():
		return

	case req := <-reqsToRegisterQueue:
		batchSize++

		// TODO update when entity key retrieval is fixed
		eKey := entity.Key(req.Data.Entity.Name)
		batch[eKey] = req

		// TODO add 1MB payload size platform limitation
		// TODO add timer trigger
		if batchSize == registerBatchSize {
			var entities []protocol.Entity
			for _, r := range batch {
				entities = append(entities, r.Data.Entity)
			}
			responses, _, errClient := client.RegisterBatchEntities(agentIDProvide().ID, entities)
			if errClient != nil {
				// TODO error handling
				break
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
					reqsRegisteredQueue <- r
				}
			}
			// reset batch
			batchSize = 0
			batch = map[entity.Key]fwrequest.EntityFwRequest{}
		}
	}
}
