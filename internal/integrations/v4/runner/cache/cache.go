package cache

import (
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
)

type Cache interface {
	GetHashes(cfgName string) map[string]struct{}
	AddDefinition(key string, definition integration.Definition) bool
	GetDefinitions(cfgName string) []integration.Definition
	ListConfigNames() []string
}

type cache struct {
	hashes      map[string]map[string]struct{}
	definitions map[string]integration.Definition
}

func New() Cache {
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
