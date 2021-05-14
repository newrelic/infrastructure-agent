package cache

import (
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/executor"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/stretchr/testify/assert"
	"testing"
)

func createIntegrationDefinition(name string) integration.Definition {
	return integration.Definition{
		Name:            name,
		Labels:          nil,
		ExecutorConfig:  executor.Config{},
		Interval:        0,
		Timeout:         0,
		ConfigTemplate:  nil,
		InventorySource: ids.PluginID{},
		WhenConditions:  nil,
		CmdChanReq:      nil,
		ConfigRequest:   nil,
	}
}

func Test_createCache(t *testing.T) {
	c := CreateCache()
	assert.NotNil(t, c)
	assert.Empty(t, c.ListConfigNames())
	assert.Empty(t, c.GetHashes("hash1"))
	assert.Empty(t, c.GetDefinitions("cfg"))

	def := createIntegrationDefinition("def1")
	def2 := createIntegrationDefinition("def2")

	added := c.AddDefinition("cfg", def)
	assert.True(t, added)
	assert.Len(t, c.ListConfigNames(), 1)
	assert.Len(t, c.GetHashes("cfg"), 1)
	assert.Len(t, c.GetDefinitions("cfg"), 1)

	added = c.AddDefinition("cfg", def)
	assert.False(t, added)
	assert.Len(t, c.ListConfigNames(), 1)
	assert.Len(t, c.GetHashes("cfg"), 1)
	assert.Len(t, c.GetDefinitions("cfg"), 1)

	added = c.AddDefinition("cfg2", def)
	assert.True(t, added)
	assert.Len(t, c.ListConfigNames(), 2)
	assert.Len(t, c.GetHashes("cfg"), 1)
	assert.Len(t, c.GetDefinitions("cfg"), 1)
	hashes := c.GetHashes("cfg2")
	assert.Contains(t, hashes, def.Hash())

	added = c.AddDefinition("cfg2", def2)
	assert.True(t, added)
	assert.Len(t, c.ListConfigNames(), 2)
	assert.Len(t, c.GetHashes("cfg2"), 2)
	assert.Len(t, c.GetDefinitions("cfg2"), 2)
	hashes = c.GetHashes("cfg2")
	assert.Contains(t, hashes, def.Hash())
	assert.Contains(t, hashes, def2.Hash())
}
