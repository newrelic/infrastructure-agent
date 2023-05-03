// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package inventory

import (
	"fmt"
	"github.com/newrelic/infrastructure-agent/internal/agent/delta"
	"github.com/newrelic/infrastructure-agent/internal/agent/types"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"sync"
	"time"
)

type PatchSenderProviderFunc func(entity.Entity) (PatchSender, error)

type EntityPatcher struct {
	m sync.Mutex

	BasePatcher
	entities map[entity.Key]struct {
		sender       PatchSender
		needsReaping bool
	}
	seenEntities map[entity.Key]struct{}

	patchSenderProviderFn PatchSenderProviderFunc
}

func NewEntityPatcher(cfg PatcherConfig, deltaStore *delta.Store, patchSenderProviderFn PatchSenderProviderFunc) Patcher {
	ep := &EntityPatcher{
		BasePatcher: BasePatcher{
			deltaStore: deltaStore,
			cfg:        cfg,
			lastClean:  time.Now(),
		},
		seenEntities: make(map[entity.Key]struct{}),

		entities: map[entity.Key]struct {
			sender       PatchSender
			needsReaping bool
		}{},
		patchSenderProviderFn: patchSenderProviderFn,
	}
	err := ep.registerEntity(cfg.AgentEntity)
	if err != nil {
		ilog.WithError(err).Error("Failed to register agent inventory entity")
	}
	return ep
}

func (ep *EntityPatcher) Send() error {
	if ep.needsCleanup() {
		ep.m.Lock()
		ep.seenEntities = make(map[entity.Key]struct{})
		ep.cleanOutdatedEntities()
		ep.m.Unlock()
	}

	ep.m.Lock()
	senders := make([]PatchSender, len(ep.entities))

	i := 0
	for _, inventory := range ep.entities {
		senders[i] = inventory.sender
		i++
	}
	ep.m.Unlock()

	for _, sender := range senders {
		if err := sender.Process(); err != nil {
			return err
		}
	}
	return nil
}

func (ep *EntityPatcher) Reap() {
	ep.m.Lock()
	defer ep.m.Unlock()

	for key, inventory := range ep.entities {
		if !inventory.needsReaping {
			continue
		}
		ep.reapEntity(key)
		inventory.needsReaping = false
	}
}

func (ep *EntityPatcher) Save(data types.PluginOutput) error {
	ep.m.Lock()
	defer ep.m.Unlock()

	if data.NotApplicable {
		return nil
	}

	if err := ep.registerEntity(data.Entity); err != nil {
		return fmt.Errorf("failed to save plugin inventory data, error: %w", err)
	}

	if err := ep.BasePatcher.save(data); err != nil {
		return fmt.Errorf("failed to save plugin inventory data, error: %w", err)
	}

	ep.seenEntities[data.Entity.Key] = struct{}{}

	e := ep.entities[data.Entity.Key]
	e.needsReaping = true
	return nil
}

func (ep *EntityPatcher) registerEntity(entity entity.Entity) error {
	if _, found := ep.entities[entity.Key]; found {
		return nil
	}

	ilog.WithField("entityKey", entity.Key.String()).
		WithField("entityID", entity.ID).Debug("Registering inventory for entity.")

	sender, err := ep.patchSenderProviderFn(entity)
	if err != nil {
		return fmt.Errorf("failed to register inventory for entity: %s, %v", entity.Key, err)
	}

	ep.entities[entity.Key] = struct {
		sender       PatchSender
		needsReaping bool
	}{sender: sender, needsReaping: true}

	return nil
}

// removes the inventory object references to free the memory, and the respective directories
func (ep *EntityPatcher) unregisterEntity(entity entity.Key) error {
	entityKey := entity.String()
	ilog.WithField("entityKey", entityKey).Debug("Unregistering inventory for entity.")

	_, ok := ep.entities[entity]
	if ok {
		delete(ep.entities, entity)
	}

	return ep.deltaStore.RemoveEntity(entityKey)
}

func (ep *EntityPatcher) cleanOutdatedEntities() {
	ilog.Debug("Triggered periodic removal of outdated entities.")
	// The entities to remove are those entities that haven't reported activity in the last period and
	// are registered in the system
	entitiesToRemove := map[entity.Key]struct{}{}

	for entityKey := range ep.entities {
		entitiesToRemove[entityKey] = struct{}{}
	}

	delete(entitiesToRemove, ep.cfg.AgentEntity.Key) // never delete local entity

	for entityKey := range ep.seenEntities {
		delete(entitiesToRemove, entityKey)
	}

	for entityKey := range entitiesToRemove {
		ilog.WithField("entityKey", entityKey.String()).Debug("Removing inventory for entity.")
		if err := ep.unregisterEntity(entityKey); err != nil {
			ilog.WithError(err).Warn("unregistering inventory for entity")
		}
	}
	// Remove folders from unregistered entities that still have folders in the data directory (e.g. from
	// previous agent executions)
	foldersToRemove, err := ep.deltaStore.ScanEntityFolders()
	if err != nil {
		ilog.WithError(err).Warn("error scanning outdated entity folders")
		// Continuing, because some entities may have been fetched despite the error
	}

	if foldersToRemove != nil {
		// We don't remove those entities that are registered
		for entityKey := range ep.entities {
			delete(foldersToRemove, helpers.SanitizeFileName(entityKey.String()))
		}
		for folder := range foldersToRemove {
			if err := ep.deltaStore.RemoveEntityFolders(folder); err != nil {
				ilog.WithField("folder", folder).WithError(err).Warn("error removing entity folder")
			}
		}
	}

	ilog.WithField("remaining", len(ep.entities)).Debug("Some entities may remain registered.")
}
