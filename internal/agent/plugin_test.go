// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package agent

import (
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/newrelic/infrastructure-agent/pkg/sample"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/hostname"
	"github.com/stretchr/testify/assert"
)

func TestPluginOutput(t *testing.T) {
	pluginOutput := NewPluginOutput(ids.PluginID{}, "", nil)
	assert.False(t, pluginOutput.NotApplicable)
	assert.NotNil(t, pluginOutput)

	pluginOutput = NewNotApplicableOutput(ids.PluginID{"a", "b"})
	assert.Equal(t, ids.PluginID{"a", "b"}, pluginOutput.Id)
	assert.True(t, pluginOutput.NotApplicable)
}

func TestPluginIDJSONMarshaling(t *testing.T) {
	id := ids.PluginID{Category: "kernel", Term: "sysctl"}
	jsonField, err := id.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, `"kernel/sysctl"`, string(jsonField))
}

func TestPluginIDJSONUnmarshaling(t *testing.T) {
	id := ids.PluginID{}
	err := id.UnmarshalJSON([]byte(`"tralari/tralara"`))
	assert.NoError(t, err)
	assert.Equal(t, ids.PluginID{Category: "tralari", Term: "tralara"}, id)
}

func TestPluginIDJSONUnmarshalingInvalid(t *testing.T) {
	id := ids.PluginID{}
	assert.Error(t, id.UnmarshalJSON([]byte("tralara")))
}

func TestPluginIDSortable(t *testing.T) {
	assert.Equal(t, "hello/guy", ids.PluginID{"hello", "guy"}.SortKey())
}

func newFakeContext(resolver hostname.Resolver) *fakeContext {
	return &fakeContext{
		resolver: resolver,
		data:     make(chan PluginOutput),
		ev:       make(chan sample.Event),
	}
}

type fakeContext struct {
	resolver hostname.Resolver
	data     chan PluginOutput
	ev       chan sample.Event
}

func (c *fakeContext) HostnameResolver() hostname.Resolver {
	return c.resolver
}

func (c *fakeContext) ActiveEntitiesChannel() chan string {
	return make(chan string, 100)
}

func (c *fakeContext) AgentIdentifier() string {
	return ""
}

func (c *fakeContext) CacheServicePids(source string, pidMap map[int]string) {}

func (c *fakeContext) Config() *config.Config {
	return &config.Config{}
}

func (c *fakeContext) GetServiceForPid(pid int) (service string, ok bool) {
	return "", false
}

func (c *fakeContext) SendData(data PluginOutput) {
	c.data <- data
}

func (c *fakeContext) SendEvent(event sample.Event, entityKey entity.Key) {
	c.ev <- event
}

func (c *fakeContext) Unregister(id ids.PluginID) {}

func (c *fakeContext) Version() string {
	return ""
}

func (c fakeContext) AddReconnecting(Plugin) {}

func (c fakeContext) Reconnect() {}
