package cache

import (
	"testing"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/stretchr/testify/assert"
)

func createIntegrationDefinition(name string) integration.Definition {
	return integration.Definition{
		Name: name,
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

func Test_definitionsChange(t *testing.T) {

	c := CreateCache()
	def := createIntegrationDefinition("def")
	def2 := createIntegrationDefinition("def2")
	def3 := createIntegrationDefinition("def3")
	def4 := createIntegrationDefinition("def4")

	assert.True(t, c.AddDefinition("cfg1", def))
	assert.True(t, c.AddDefinition("cfg1", def2))
	assert.True(t, c.AddDefinition("cfg1", def3))

	definitionsList := c.GetDefinitions("cfg1")
	for _, def := range c.GetDefinitions("cfg1") {
		assert.Contains(t, []string{"def", "def2", "def3"}, def.Name)
	}
	assert.Len(t, definitionsList, 3)

	cfgDefinitions := c.Take("cfg1")
	assert.False(t, cfgDefinitions.Add(def))
	assert.False(t, cfgDefinitions.Add(def2))
	assert.True(t, cfgDefinitions.Add(def4))

	toBeDeleted := c.Apply(cfgDefinitions)
	definitionsList = c.GetDefinitions("cfg1")
	assert.Len(t, definitionsList, 3)
	for _, def := range c.GetDefinitions("cfg1") {
		assert.Contains(t, []string{"def", "def2", "def4"}, def.Name)
	}
	assert.Equal(t, []string{def3.Hash()}, toBeDeleted)
}
