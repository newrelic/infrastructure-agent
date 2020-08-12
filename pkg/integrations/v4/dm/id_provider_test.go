package dm

import (
	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
	"time"
)

func TestIdProvider_Entities_MemoryFirst(t *testing.T) {

	agentIdn := entity.Identity{ID: 13}

	registerClient := &mockedRegisterClient{}
	registerClient.
		On("RegisterBatchEntities", agentIdn.ID, mock.Anything).
		Return([]identityapi.RegisterEntityResponse{}, time.Second, nil)

	cache := RegisteredEntitiesNameIDMap{
		"remote_entity_flex":  6543,
		"remote_entity_nginx": 1234,
	}

	entities := []protocol.Entity{
		{Name: "remote_entity_flex"},
		{Name: "remote_entity_nginx"},
	}

	idProvider := NewIDProvider(registerClient)

	idProvider.cache = cache
	idProvider.Entities(agentIdn, entities)

	registerClient.AssertNotCalled(t, "RegisterBatchEntities")
}

func TestIdProvider_Entities_OneCachedAnotherNot(t *testing.T) {

	agentIdn := entity.Identity{ID: 13}

	entitiesForRegisterClient := []protocol.Entity{
		{
			Name: "remote_entity_nginx",
		},
	}

	registerClientResponse := []identityapi.RegisterEntityResponse{
		{
			ID:   1234,
			Key:  "remote_entity_nginx_Key",
			Name: "remote_entity_nginx",
		},
	}

	registerClient := &mockedRegisterClient{}
	registerClient.
		On("RegisterBatchEntities", mock.Anything, mock.Anything).
		Return(registerClientResponse, time.Second, nil)

	cache := RegisteredEntitiesNameIDMap{
		"remote_entity_flex": 6543,
	}

	entities := []protocol.Entity{
		{Name: "remote_entity_flex"},
		{Name: "remote_entity_nginx"},
	}

	idProvider := NewIDProvider(registerClient)

	idProvider.cache = cache
	registeredEntities, unregisteredEntities := idProvider.Entities(agentIdn, entities)

	assert.Empty(t, unregisteredEntities)
	assert.Len(t, registeredEntities, 2)

	registerClient.AssertCalled(t, "RegisterBatchEntities", agentIdn.ID, entitiesForRegisterClient)
}
