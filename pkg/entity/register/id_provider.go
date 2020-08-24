package register

import (
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
)

type RegisteredEntitiesNameToID map[string]entity.ID
type UnregisteredEntitiesNamed map[string]UnregisteredEntity
type reason string

type UnregisteredEntity struct {
	Reason reason
	Err    error
	Entity identityapi.RegisterEntity
}

type UnregisteredEntities []UnregisteredEntity

const ReasonClientError = "Identity client error"
const ReasonEntityError = "Entity error"

func newUnregisteredEntity(entity identityapi.RegisterEntity, reason reason, err error) UnregisteredEntity {
	return UnregisteredEntity{
		Entity: entity,
		Reason: reason,
		Err:    err,
	}
}

type IDProvider interface {
	ResolveEntities(agentIdn entity.Identity, entities []protocol.Entity) (registeredEntities RegisteredEntitiesNameToID, unregisteredEntities UnregisteredEntities)
}

type CachedIDProvider struct {
	client               identityapi.RegisterClient
	cache                RegisteredEntitiesNameToID
	unregisteredEntities UnregisteredEntitiesNamed
}

func NewCachedIDProvider(client identityapi.RegisterClient) *CachedIDProvider {
	cache := make(RegisteredEntitiesNameToID)
	unregisteredEntities := make(UnregisteredEntitiesNamed)
	return &CachedIDProvider{
		client:               client,
		cache:                cache,
		unregisteredEntities: unregisteredEntities,
	}
}

func (p *CachedIDProvider) ResolveEntities(agentIdn entity.Identity, entities []protocol.Entity) (registeredEntities RegisteredEntitiesNameToID, unregisteredEntities UnregisteredEntities) {
	unregisteredEntities = make(UnregisteredEntities, 0)
	registeredEntities = make(RegisteredEntitiesNameToID, 0)
	entitiesToRegister := make([]identityapi.RegisterEntity, 0)

	for _, e := range entities {
		if foundID, ok := p.cache[e.Name]; ok {
			registeredEntities[e.Name] = foundID
		} else {
			registerEntity := identityapi.RegisterEntity{
				Name:        e.Name,
				DisplayName: e.DisplayName,
				EntityType:  e.Type,
				Metadata:    e.Metadata,
			}
			entitiesToRegister = append(entitiesToRegister, registerEntity)
		}
	}

	if len(entitiesToRegister) == 0 {
		return registeredEntities, unregisteredEntities
	}

	response, _, errClient := p.client.RegisterBatchEntities(
		agentIdn.ID,
		entitiesToRegister)

	nameToEntity := make(map[string]identityapi.RegisterEntity, len(entitiesToRegister))

	for i := range entitiesToRegister {
		nameToEntity[entitiesToRegister[i].Name] = entitiesToRegister[i]
	}

	if errClient != nil {
		for i := range entitiesToRegister {
			unregisteredEntity := newUnregisteredEntity(entitiesToRegister[i], ReasonClientError, errClient)
			p.unregisteredEntities[entitiesToRegister[i].Name] = unregisteredEntity
			unregisteredEntities = append(unregisteredEntities, unregisteredEntity)
		}
	} else {
		for i := range response {
			if response[i].Err != "" {
				unregisteredEntity := newUnregisteredEntity(nameToEntity[response[i].Name], ReasonEntityError, fmt.Errorf(response[i].Err))
				p.unregisteredEntities[response[i].Name] = unregisteredEntity
				unregisteredEntities = append(unregisteredEntities, unregisteredEntity)
				continue
			}

			p.cache[string(response[i].Key)] = response[i].ID
			registeredEntities[string(response[i].Key)] = response[i].ID
		}
	}

	return registeredEntities, unregisteredEntities
}
