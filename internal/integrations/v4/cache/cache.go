package cache

import (
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
)

type Cache interface {
	GetHashes(cfgName string) map[string]struct{}
	AddDefinition(key string, definition integration.Definition) bool
	GetDefinitions(cfgName string) []integration.Definition
	ListConfigNames() []string
	Apply(cfgDefinitions *ConfigDefinitions) []string
	Take(cfgName string) *ConfigDefinitions
}

type ConfigDefinitions struct {
	cfgName string
	added   map[string]integration.Definition
	current map[string]struct{}
}

func (cfgDefinition *ConfigDefinitions) Add(def integration.Definition) *ConfigDefinitions{
	cfgDefinition.added[def.Hash()] = def
	return cfgDefinition
}



type cache struct {
	hashes      map[string]map[string]struct{}
	definitions map[string]integration.Definition
}

func CreateCache() Cache {
	return &cache{
		hashes:      make(map[string]map[string]struct{}),
		definitions: make(map[string]integration.Definition),
	}
}

func (c *cache) AddDefinition(cfgName string, definition integration.Definition) bool {
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
	output := make([]string, len(c.hashes))
	i := 0
	for cfgName := range c.hashes {
		output[i] = cfgName
		i++
	}
	return output
}

func (c *cache) GetHashes(cfgName string) map[string]struct{} {
	return c.hashes[cfgName]
}

func (c *cache) GetDefinitions(cfgName string) []integration.Definition {
	cfg := c.hashes[cfgName]
	output := make([]integration.Definition, len(cfg))
	i := 0
	for hash := range cfg {
		output[i] = c.definitions[hash]
		i++
	}
	return output
}

func (c *cache) Apply(cfgDefinitions *ConfigDefinitions) []string {
	toBeDeleted:=make([]string,0)
	for hash, definition := range cfgDefinitions.added {
		if _, ok := c.hashes[cfgDefinitions.cfgName][hash]; !ok {
			c.AddDefinition(cfgDefinitions.cfgName, definition)
		}
	}
	for hash := range cfgDefinitions.current {
		if _, ok := cfgDefinitions.added[hash]; !ok {
			delete(c.definitions, hash)
			delete(c.hashes[cfgDefinitions.cfgName], hash)
			toBeDeleted=append(toBeDeleted,hash)
		}
	}
	return toBeDeleted
}

func (c *cache) Take(cfgName string) *ConfigDefinitions{
	return &ConfigDefinitions{
		cfgName: cfgName,
		added:   make(map[string]integration.Definition),
		current: c.GetHashes(cfgName),
	}
}