package dm

import (
	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
)

// TODO: move into entity pkg? entity.Name
type entityName string

type EntityIDClientResponse map[string]identityapi.RegisterEntityResponse

type RegisteredEntitiesNameIDMap map[string]entity.ID

type UnregisteredEntities []UnregisteredEntity
type reason string

type UnregisteredEntity struct {
	Reason reason
	Err    error
	Entity protocol.Entity
}

func newUnregisteredEntity(entity protocol.Entity, reason reason, err error) UnregisteredEntity {
	return UnregisteredEntity{
		Entity: entity,
		Reason: reason,
		Err:    err,
	}
}

type idProvider struct {
	client identityapi.RegisterClient
	cache  RegisteredEntitiesNameIDMap
}

func NewIDProvider(client identityapi.RegisterClient) idProvider {
	cache := make(RegisteredEntitiesNameIDMap)

	return idProvider{
		client:             client,
		cache:              cache,
	}
}

func (p *idProvider) Entities(agentIdn entity.Identity, entities []protocol.Entity) (registeredEntities RegisteredEntitiesNameIDMap, unregisteredEntities UnregisteredEntities) {

	unregisteredEntities = make(UnregisteredEntities, 0)

	registeredEntities = make(RegisteredEntitiesNameIDMap, 0)
	entitiesToRegister := make([]protocol.Entity, 0)

	for _, e := range entities {
		if foundID, ok := p.cache[e.Name]; ok {
			registeredEntities[e.Name] = foundID
		} else {
			entitiesToRegister = append(entitiesToRegister, e)
		}
	}

	if len(entitiesToRegister) > 0 {
		response, _, _ := p.client.RegisterBatchEntities(
			agentIdn.ID,
			entitiesToRegister)

		for i := range response {
			p.cache[string(response[i].Key)] = response[i].ID
			registeredEntities[string(response[i].Key)] = response[i].ID
		}
	}

	return registeredEntities, unregisteredEntities
}