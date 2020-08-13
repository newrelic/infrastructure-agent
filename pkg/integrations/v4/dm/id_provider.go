package dm

import (
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
)

type EntityIDClientResponse map[string]identityapi.RegisterEntityResponse

type RegisteredEntitiesNameIDMap map[string]entity.ID
type UnregisteredEntitiesNamed map[string]UnregisteredEntity

type UnregisteredEntities []UnregisteredEntity
type reason string

const reasonClientError = "Identity client error"
const reasonEntityError = "Entity error"

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

type idProviderInterface interface {
	Entities(agentIdn entity.Identity, entities []protocol.Entity) (registeredEntities RegisteredEntitiesNameIDMap, unregisteredEntities UnregisteredEntities)
}

// change to interface
type idProvider struct {
	client identityapi.RegisterClient
	cache  RegisteredEntitiesNameIDMap
	unregisteredEntities UnregisteredEntitiesNamed
}

func NewIDProvider(client identityapi.RegisterClient) *idProvider {
	cache := make(RegisteredEntitiesNameIDMap)
	unregisteredEntities := make(UnregisteredEntitiesNamed)
	return &idProvider{
		client:               client,
		cache:                cache,
		unregisteredEntities: unregisteredEntities,
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
		response, _, errClient := p.client.RegisterBatchEntities(
			agentIdn.ID,
			entitiesToRegister)

		type nameToEntityType map[string]protocol.Entity
		nameToEntity := make(nameToEntityType, len(entitiesToRegister))

		for i := range entitiesToRegister{
			nameToEntity[entitiesToRegister[i].Name] = entitiesToRegister[i]
		}

		if errClient != nil{
			for i := range entitiesToRegister{
				unregisteredEntity := newUnregisteredEntity(entitiesToRegister[i], reasonClientError, errClient)
				p.unregisteredEntities[entitiesToRegister[i].Name] = unregisteredEntity
				unregisteredEntities = append(unregisteredEntities, unregisteredEntity)
			}
		}else{
			for i := range response {
				if response[i].Err != "" {
					unregisteredEntity := newUnregisteredEntity(nameToEntity[response[i].Name], reasonEntityError, fmt.Errorf(response[i].Err))
					p.unregisteredEntities[response[i].Name] = unregisteredEntity
					unregisteredEntities = append(unregisteredEntities, unregisteredEntity)
					continue
				}

				p.cache[string(response[i].Key)] = response[i].ID
				registeredEntities[string(response[i].Key)] = response[i].ID
			}
		}
	}

	return registeredEntities, unregisteredEntities
}
