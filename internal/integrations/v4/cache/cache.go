// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package cache

import (
	"sync"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
)

// Cache stores integrations definitions grouped by a Config Name.
type Cache interface {
	GetDefinitions(cfgName string) []integration.Definition
	ListConfigNames() []string
	ApplyConfig(cfgDefinitions *ConfigDefinitions) []integration.Definition
	TakeConfig(cfgName string) *ConfigDefinitions
}

type ConfigDefinitions struct {
	cfgName string
	added   map[string]integration.Definition
	current map[string]struct{}
}

func (cfgDefinition *ConfigDefinitions) Add(def integration.Definition) bool {
	dh := def.Hash()
	cfgDefinition.added[dh] = def
	_, ok := cfgDefinition.current[dh]
	return !ok
}

// cache implements Cache to store integrations definitions by config protocols request names
type cache struct {
	lock        sync.RWMutex
	hashes      map[string]map[string]struct{}
	definitions map[string]integration.Definition
}

// CreateCache initialize and return an empty cache
func CreateCache() Cache {
	return &cache{
		hashes:      make(map[string]map[string]struct{}),
		definitions: make(map[string]integration.Definition),
	}
}

// addDefinition adds a integration definition to a cfg name group, returns false if already exists.
func (c *cache) addDefinition(cfgName string, definition integration.Definition) bool {
	hash := definition.Hash()
	if _, ok := c.hashes[cfgName][hash]; ok {
		return false
	}
	if _, ok := c.hashes[cfgName]; !ok {
		c.hashes[cfgName] = make(map[string]struct{})
	}
	c.hashes[cfgName][hash] = struct{}{}
	c.definitions[hash] = definition
	return true
}

// ListConfigNames returns a list of config names
func (c *cache) ListConfigNames() []string {
	c.lock.RLock()
	defer c.lock.RUnlock()
	output := make([]string, len(c.hashes))
	i := 0
	for cfgName := range c.hashes {
		output[i] = cfgName
		i++
	}
	return output
}

func (c *cache) getHashes(cfgName string) map[string]struct{} {
	return c.hashes[cfgName]
}

// GetDefinitions returns a list of integration definitions for a particular config name.
func (c *cache) GetDefinitions(cfgName string) []integration.Definition {
	c.lock.RLock()
	defer c.lock.RUnlock()
	cfg := c.hashes[cfgName]
	output := make([]integration.Definition, len(cfg))
	i := 0
	for hash := range cfg {
		output[i] = c.definitions[hash]
		i++
	}
	return output
}

// ApplyConfig sync the integrations definitions for a particular config name with the added definitions in cfgDefinitions.
// returns a list of removed definitions for the config name.
func (c *cache) ApplyConfig(cfgDefinitions *ConfigDefinitions) []integration.Definition {
	c.lock.Lock()
	defer c.lock.Unlock()
	toBeDeleted := make([]integration.Definition, 0)
	for hash, definition := range cfgDefinitions.added {
		if _, ok := c.hashes[cfgDefinitions.cfgName][hash]; !ok {
			c.addDefinition(cfgDefinitions.cfgName, definition)
		}
	}
	for hash := range cfgDefinitions.current {
		if _, ok := cfgDefinitions.added[hash]; !ok {
			toBeDeleted = append(toBeDeleted, c.definitions[hash])
			delete(c.definitions, hash)
			delete(c.hashes[cfgDefinitions.cfgName], hash)
		}
	}
	return toBeDeleted
}

// TakeConfig returns a ConfigDefinitions initialized for a particular config name
func (c *cache) TakeConfig(cfgName string) *ConfigDefinitions {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return &ConfigDefinitions{
		cfgName: cfgName,
		added:   make(map[string]integration.Definition),
		current: c.getHashes(cfgName),
	}
}
