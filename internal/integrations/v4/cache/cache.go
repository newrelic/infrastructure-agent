package cache

import (
	"sync"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
)

type Cache interface {
	GetDefinitions(cfgName string) []integration.Definition
	ListConfigNames() []string
	ApplyConfig(cfgDefinitions *ConfigDefinitions) []string
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

type cache struct {
	lock        sync.RWMutex
	hashes      map[string]map[string]struct{}
	definitions map[string]integration.Definition
}

func CreateCache() Cache {
	return &cache{
		hashes:      make(map[string]map[string]struct{}),
		definitions: make(map[string]integration.Definition),
	}
}

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

func (c *cache) ApplyConfig(cfgDefinitions *ConfigDefinitions) []string {
	c.lock.Lock()
	defer c.lock.Unlock()
	toBeDeleted := make([]string, 0)
	for hash, definition := range cfgDefinitions.added {
		if _, ok := c.hashes[cfgDefinitions.cfgName][hash]; !ok {
			c.addDefinition(cfgDefinitions.cfgName, definition)
		}
	}
	for hash := range cfgDefinitions.current {
		if _, ok := cfgDefinitions.added[hash]; !ok {
			delete(c.definitions, hash)
			delete(c.hashes[cfgDefinitions.cfgName], hash)
			toBeDeleted = append(toBeDeleted, hash)
		}
	}
	return toBeDeleted
}

func (c *cache) TakeConfig(cfgName string) *ConfigDefinitions {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return &ConfigDefinitions{
		cfgName: cfgName,
		added:   make(map[string]integration.Definition),
		current: c.getHashes(cfgName),
	}
}
