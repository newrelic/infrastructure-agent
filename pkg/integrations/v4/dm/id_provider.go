// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package dm

import (
	"context"
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
type unregisteredEntityListWithWait struct {
	entities  unregisteredEntityList
	waitGroup *sync.WaitGroup
}

type entityListToRegisterWithWait struct {
	entities  []protocol.Entity
	waitGroup *sync.WaitGroup
}

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
	ResolveEntities(entities []protocol.Entity) (registeredEntitiesNameToID, unregisteredEntityListWithWait)
}

type cachedIdProvider struct {
	client               identityapi.RegisterClient
	agentIdentity        func() entity.Identity
	cache                registeredEntitiesNameToID
	unregisteredEntities unregisteredEntitiesNamed
	cacheMutex           *sync.Mutex
	toRegisterQueue      chan entityListToRegisterWithWait
}

func NewCachedIDProvider(client identityapi.RegisterClient, agentIdentity func() entity.Identity, closeContext context.Context) *cachedIdProvider {
	cache := make(registeredEntitiesNameToID)
	unregisteredEntities := make(unregisteredEntitiesNamed)
	cacheMutex := &sync.Mutex{}
	provider := cachedIdProvider{
		client:               client,
		agentIdentity:        agentIdentity,
		cache:                cache,
		unregisteredEntities: unregisteredEntities,
		cacheMutex:           cacheMutex,
		toRegisterQueue:      make(chan entityListToRegisterWithWait, 10), // TODO adjust buffer
	}

	go provider.registerEntitiesWorker(closeContext)

	return &provider
}

func (p *cachedIdProvider) ResolveEntities(entities []protocol.Entity) (registeredEntitiesNameToID, unregisteredEntityListWithWait) {
	unregisteredEntities := make(unregisteredEntityList, 0)
	registeredEntities := make(registeredEntitiesNameToID, 0)

	entitiesToRegister := make([]protocol.Entity, 0)

	// add error cache checking
	p.cacheMutex.Lock()
	for _, e := range entities {
		if foundID, ok := p.cache[e.Name]; ok {
			registeredEntities[e.Name] = foundID
		} else if foundUnregisteredEntities, ok := p.unregisteredEntities[e.Name]; ok {
			unregisteredEntities = append(unregisteredEntities, foundUnregisteredEntities)
		} else {
			unregisteredEntities = append(unregisteredEntities, newUnregisteredEntity(e, reasonNotInCache, nil))
			entitiesToRegister = append(entitiesToRegister, e)
		}
	}
	p.cacheMutex.Unlock()

	wg := &sync.WaitGroup{}
	wg.Add(1)

	if len(entitiesToRegister) != 0 {
		p.toRegisterQueue <- entityListToRegisterWithWait{
			entities:  entitiesToRegister,
			waitGroup: wg,
		}
	}

	unregisteredEntitiesWithWait := unregisteredEntityListWithWait{
		entities:  unregisteredEntities,
		waitGroup: wg,
	}

	return registeredEntities, unregisteredEntitiesWithWait
}

// todo add backoff strategy
func (p *cachedIdProvider) registerEntitiesWorker(closeContext context.Context) {
	type nameToEntityType map[string]protocol.Entity

	for {
		select {
		case <-closeContext.Done():
			return
		case entitiesToRegisterWithWait := <-p.toRegisterQueue:
			response, _, errClient := p.client.RegisterBatchEntities(
				p.agentIdentity().ID,
				entitiesToRegisterWithWait.entities)

			nameToEntity := make(nameToEntityType, len(entitiesToRegisterWithWait.entities))
			for i := range entitiesToRegisterWithWait.entities {
				nameToEntity[entitiesToRegisterWithWait.entities[i].Name] = entitiesToRegisterWithWait.entities[i]
			}

			p.cacheMutex.Lock()
			if errClient != nil {
				for i := range entitiesToRegisterWithWait.entities {
					p.unregisteredEntities[entitiesToRegisterWithWait.entities[i].Name] = newUnregisteredEntity(entitiesToRegisterWithWait.entities[i], reasonClientError, errClient)
				}
			} else {
				for i := range response {
					if response[i].Err != "" {
						p.unregisteredEntities[response[i].Name] = newUnregisteredEntity(nameToEntity[response[i].Name], reasonEntityError, fmt.Errorf(response[i].Err))
						continue
					}

					p.cache[string(response[i].Key)] = response[i].ID
				}
			}
			p.cacheMutex.Unlock()
			entitiesToRegisterWithWait.waitGroup.Done()
		}
	}
}
