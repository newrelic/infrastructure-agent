package cache

import (
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/helpers/lru"
	"sync"
)

var once sync.Once
var instance *cache

type Cache interface {
	AddDefinition(key string, definition *integration.Definition)
	GetDefinition(cfgName string) *integration.Definition
}

type cache struct {
	hashes       *lru.Cache
	descriptions *lru.Cache
}

func Get() Cache {
	once.Do(func() {
		instance = &cache{
			hashes: lru.New(),
		}
	})
	return instance
}

/**
Add a method to the cache to add a integration.Definition to a given config_name
input: config_name, integration.Definition
output: added (bool)
If the integration exists already for a given config_name then return added=false. Otherwise, add the integration.Definition to that config_name entry and return added=true
Use the previous method ToHash() to compare integration.Definitions
*/
func (c *cache) AddDefinition(cfgName string, definition *integration.Definition) bool {
	if _, ok := c.hashes.Get(cfgName); ok {
		return false
	}
	c.hashes.Add(cfgName, definition)
	return true
}

func (c *cache) GetDefinition(cfgName string) *integration.Definition {
	if value, ok := c.hashes.Get(cfgName); ok {
		return value.(*integration.Definition)
	}
	return nil
}
