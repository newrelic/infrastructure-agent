// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package dm

import (
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
	"sync"
)

type registeredEntitiesNameToID map[string]entity.ID
type unregisteredEntitiesNamed map[string]unregisteredEntity
type reason string

type unregisteredEntity struct {
	Reason reason
	Err    error
	Entity protocol.Entity
}

type unregisteredEntityList []unregisteredEntity

const reasonClientError = "Identity client error"
const reasonEntityError = "Entity error"
const reasonNotInCache = "Entity not cached yet"

func newUnregisteredEntity(entity protocol.Entity, reason reason, err error) unregisteredEntity {
	return unregisteredEntity{
		Entity: entity,
		Reason: reason,
		Err:    err,
	}
}

type idProviderInterface interface {
	ResolveEntities(entities []protocol.Entity) (registeredEntitiesNameToID, unregisteredEntityList)
}

type cachedIdProvider struct {
	client               identityapi.RegisterClient
	agentIdentity        func() entity.Identity
	cache                registeredEntitiesNameToID
	unregisteredEntities unregisteredEntitiesNamed
	cacheMutex           *sync.Mutex
	toRegisterQueue      chan []protocol.Entity
}

func NewCachedIDProvider(client identityapi.RegisterClient, agentIdentity func() entity.Identity) *cachedIdProvider {
	cache := make(registeredEntitiesNameToID)
	unregisteredEntities := make(unregisteredEntitiesNamed)
	cacheMutex := &sync.Mutex{}
	provider := cachedIdProvider{
		client:               client,
		agentIdentity:        agentIdentity,
		cache:                cache,
		unregisteredEntities: unregisteredEntities,
		cacheMutex:           cacheMutex,
		toRegisterQueue:      make(chan []protocol.Entity, 10), // TODO adjust buffer
	}

	go provider.registerEntitiesWorker()

	return &provider
}

func (p *cachedIdProvider) ResolveEntities(entities []protocol.Entity) (registeredEntitiesNameToID, unregisteredEntityList) {
	unregisteredEntities := make(unregisteredEntityList, 0)
	registeredEntities := make(registeredEntitiesNameToID, 0)
	entitiesToRegister := make([]protocol.Entity, 0)

	// add error cache checking

	for _, e := range entities {
		p.cacheMutex.Lock()
		if foundID, ok := p.cache[e.Name]; ok {
			registeredEntities[e.Name] = foundID
		} else if foundUnregisteredEntities, ok := p.unregisteredEntities[e.Name]; ok {
			unregisteredEntities = append(unregisteredEntities, foundUnregisteredEntities)
		} else {
			unregisteredEntities = append(unregisteredEntities, newUnregisteredEntity(e, reasonNotInCache, nil))
			entitiesToRegister = append(entitiesToRegister, e)
		}
		p.cacheMutex.Unlock()
	}

	if len(entitiesToRegister) != 0 {
		p.toRegisterQueue <- entitiesToRegister
	}

	return registeredEntities, unregisteredEntities
}

// todo add close chan logic
func (p *cachedIdProvider) registerEntitiesWorker() {
	type nameToEntityType map[string]protocol.Entity

	for {
		select {
		case entitiesToRegister := <-p.toRegisterQueue:
			response, _, errClient := p.client.RegisterBatchEntities(
				p.agentIdentity().ID,
				entitiesToRegister)

			nameToEntity := make(nameToEntityType, len(entitiesToRegister))
			for i := range entitiesToRegister {
				nameToEntity[entitiesToRegister[i].Name] = entitiesToRegister[i]
			}

			if errClient != nil {
				for i := range entitiesToRegister {
					p.cacheMutex.Lock()
					p.unregisteredEntities[entitiesToRegister[i].Name] = newUnregisteredEntity(entitiesToRegister[i], reasonClientError, errClient)
					p.cacheMutex.Unlock()
				}
			} else {
				for i := range response {
					if response[i].Err != "" {
						p.cacheMutex.Lock()
						p.unregisteredEntities[response[i].Name] = newUnregisteredEntity(nameToEntity[response[i].Name], reasonEntityError, fmt.Errorf(response[i].Err))
						p.cacheMutex.Unlock()
						continue
					}

					p.cacheMutex.Lock()
					p.cache[string(response[i].Key)] = response[i].ID
					p.cacheMutex.Unlock()
				}
			}
		}
	}
}