// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package testing

import (
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/newrelic/infrastructure-agent/pkg/sample"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/hostname"

	. "gopkg.in/check.v1"
)

type MockAgent struct {
	ch         chan agent.PluginOutput
	registered bool
	cfg        *config.Config
	entities   chan string
	resolver   hostname.Resolver
}

func (m *MockAgent) HostnameResolver() hostname.Resolver {
	return m.resolver
}

func NewMockAgent() *MockAgent {
	return &MockAgent{
		registered: true,
		ch:         make(chan agent.PluginOutput, 1),
		cfg: &config.Config{
			SupervisorRefreshSec: 1,
			SupervisorRpcSocket:  "/tmp/supervisor.sock.test",
		},
		entities: make(chan string, 1000),
		resolver: hostname.CreateResolver("", "", true),
	}
}

func (m *MockAgent) ActiveEntitiesChannel() chan string {
	return m.entities
}

func (self *MockAgent) WithConfig(cfg *config.Config) *MockAgent {
	self.cfg = cfg
	return self
}

func (self *MockAgent) GetData(c *C) (output agent.PluginOutput) {
	select {
	case output = <-self.ch:
	case <-time.After(50 * time.Millisecond):
		c.Fatalf("Timeout waiting on agent data from channel %#v", self.ch)
	}
	return
}

func (self *MockAgent) SendData(data agent.PluginOutput) {
	self.ch <- data
}

func (self *MockAgent) SendEvent(event sample.Event, entityKey entity.Key) {
	// Not implemented yet
}

func (self *MockAgent) Unregister(id ids.PluginID) {
	self.registered = false
	self.ch <- agent.NewNotApplicableOutput(id)
}

func (self *MockAgent) Config() *config.Config {
	return self.cfg
}

func (self *MockAgent) AgentIdentifier() string {
	return ""
}

func (self *MockAgent) Version() string {
	return "mock"
}

func (self *MockAgent) CacheServicePids(source string, pidMap map[int]string) {
	return
}

func (self *MockAgent) GetServiceForPid(pid int) (service string, ok bool) {
	return
}

func (self *MockAgent) CloudDetector() *cloud.Detector {
	return cloud.NewDetector(true, 0, 0, 0, false)
}

func (m *MockAgent) AddReconnecting(agent.Plugin) {}

func (m *MockAgent) Reconnect() {}

func (m *MockAgent) IDLookup() agent.IDLookup {
	idLookupTable := make(agent.IDLookup)
	idLookupTable[sysinfo.HOST_SOURCE_HOSTNAME_SHORT] = "short_hostname"
	return idLookupTable
}
