package test

import (
	"math/rand"
	"net/http"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi"
	"github.com/newrelic/infrastructure-agent/pkg/backend/inventoryapi"
	"github.com/newrelic/infrastructure-agent/pkg/backend/state"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
)

type EmptyRegisterClient struct{}

func (icc *EmptyRegisterClient) RegisterEntitiesRemoveMe(agentEntityID entity.ID, entities []identityapi.RegisterEntity) (r []identityapi.RegisterEntityResponse, retryAfter time.Duration, err error) {
	return
}

func (icc *EmptyRegisterClient) RegisterBatchEntities(agentEntityID entity.ID, entities []entity.Fields) (r []identityapi.RegisterEntityResponse, retryAfter time.Duration, err error) {
	return
}

func (icc *EmptyRegisterClient) RegisterEntity(agentEntityID entity.ID, entity entity.Fields) (resp identityapi.RegisterEntityResponse, err error) {
	return
}

type IncrementalRegister struct {
	state state.Register
}

func NewIncrementalRegister() identityapi.RegisterClient {
	return &IncrementalRegister{state: state.RegisterHealthy}
}

func NewRetryAfterRegister() identityapi.RegisterClient {
	return &IncrementalRegister{state: state.RegisterRetryAfter}
}

func NewRetryBackoffRegister() identityapi.RegisterClient {
	return &IncrementalRegister{state: state.RegisterRetryBackoff}
}

func (r *IncrementalRegister) RegisterBatchEntities(agentEntityID entity.ID, entities []entity.Fields) (batchResponse []identityapi.RegisterEntityResponse, retryAfter time.Duration, err error) {
	if r.state == state.RegisterRetryAfter {
		retryAfter = 1 * time.Second
		err = inventoryapi.NewIngestError("ingest service rejected the register step", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), "")
		return
	} else if r.state == state.RegisterRetryBackoff {
		err = inventoryapi.NewIngestError("ingest service rejected the register step", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), "")
		return
	}

	var i entity.ID
	for _, e := range entities {
		i++
		eKey := entity.Key(e.Name) // TODO use host.ResolveUniqueEntityKey instead!
		batchResponse = append(batchResponse, identityapi.RegisterEntityResponse{ID: i, Key: eKey})
	}

	return
}

func (r *IncrementalRegister) RegisterEntitiesRemoveMe(agentEntityID entity.ID, entities []identityapi.RegisterEntity) (responseKeys []identityapi.RegisterEntityResponse, retryAfter time.Duration, err error) {
	if r.state == state.RegisterRetryAfter {
		retryAfter = 1 * time.Second
		err = inventoryapi.NewIngestError("ingest service rejected the register step", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), "")
		return
	} else if r.state == state.RegisterRetryBackoff {
		err = inventoryapi.NewIngestError("ingest service rejected the register step", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), "")
		return
	}

	var i entity.ID
	for _, e := range entities {
		i++
		responseKeys = append(responseKeys, identityapi.RegisterEntityResponse{ID: i, Key: e.Key})
	}

	return
}

func (r *IncrementalRegister) RegisterEntity(agentEntityID entity.ID, ent entity.Fields) (identityapi.RegisterEntityResponse, error) {
	return identityapi.RegisterEntityResponse{
		ID:  entity.ID(rand.Int63n(100000)),
		Key: entity.Key(ent.Name),
	}, nil
}
