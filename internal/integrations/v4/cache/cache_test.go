// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"testing"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	assert.Empty(t, c.GetDefinitions("cfg"))

	def := createIntegrationDefinition("def")
	initialCfg := &ConfigDefinitions{
		cfgName: "cfg",
		added: map[string]integration.Definition{
			def.Hash(): def,
		},
	}

	assert.Empty(t, c.ApplyConfig(initialCfg))
	definitions := c.GetDefinitions("cfg")
	require.Len(t, definitions, 1)
	assert.Equal(t, "def", definitions[0].Name)

	cd := c.TakeConfig("cfg")
	for hash := range cd.current {
		assert.Equal(t, def.Hash(), hash)
	}

}

func Test_definitionsChange(t *testing.T) {
	c := CreateCache()
	def := createIntegrationDefinition("def")
	def2 := createIntegrationDefinition("def2")
	def3 := createIntegrationDefinition("def3")
	def4 := createIntegrationDefinition("def4")

	initialCfg := &ConfigDefinitions{
		cfgName: "cfg1",
		added: map[string]integration.Definition{
			def.Hash():  def,
			def2.Hash(): def2,
			def3.Hash(): def3,
		},
	}
	assert.Empty(t, c.ApplyConfig(initialCfg))
	definitionsList := c.GetDefinitions("cfg1")
	for _, def := range c.GetDefinitions("cfg1") {
		assert.Contains(t, []string{"def", "def2", "def3"}, def.Name)
	}
	assert.Len(t, definitionsList, 3)

	cfgDefinitions := c.TakeConfig("cfg1")
	assert.False(t, cfgDefinitions.Add(def))
	assert.False(t, cfgDefinitions.Add(def2))
	assert.True(t, cfgDefinitions.Add(def4))

	toBeDeleted := c.ApplyConfig(cfgDefinitions)
	definitionsList = c.GetDefinitions("cfg1")
	assert.Len(t, definitionsList, 3)
	for _, def := range c.GetDefinitions("cfg1") {
		assert.Contains(t, []string{"def", "def2", "def4"}, def.Name)
	}
	require.Len(t, toBeDeleted, 1)
	assert.Equal(t, def3.Hash(), toBeDeleted[0].Hash())
}
