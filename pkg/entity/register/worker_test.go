package register

import (
	"context"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/fwrequest"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
	"gotest.tools/assert"
)

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
}

func (c *fakeClient) RegisterBatchEntities(agentEntityID entity.ID, entities []entity.Fields) (r []identityapi.RegisterEntityResponse, t time.Duration, err error) {
	r = []identityapi.RegisterEntityResponse{}
	for _, id := range c.ids {
		r = append(r, identityapi.RegisterEntityResponse{ID: id})
	}
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

	w := NewWorker(agentIdentity, newClientReturning(123), reqsToRegisterQueue, reqsRegisteredQueue, 2, 50*time.Millisecond)

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
	w := NewWorker(agentIdentity, newClientReturning(ids...), reqsToRegisterQueue, reqsRegisteredQueue, 2, 50*time.Millisecond)

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
