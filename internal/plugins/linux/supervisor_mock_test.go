// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux
// +build linux

package linux

import (
	"fmt"
	"testing"
	"time"

	testHelper "github.com/newrelic/infrastructure-agent/internal/plugins/testing"

	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	. "gopkg.in/check.v1"
)

// Register test suite.
func TestSupervisorMock(t *testing.T) {
	t.Parallel()
}

type SuperMockSuite struct {
	agent *testHelper.MockAgent
}

var _ = Suite(&SuperMockSuite{})

func (s *SuperMockSuite) SetUpTest(c *C) {
	s.agent = testHelper.NewMockAgent()
}

func (s *SuperMockSuite) TestSupervisorAllGood(c *C) {
	s.NewPlugin(ids.PluginID{"supervisor", "good"}, &MockSupervisor{}, c)
	x := s.agent.GetData(c)
	c.Assert(x.NotApplicable, Equals, false)
	c.Assert(x.Data, NotNil)
	c.Assert(x.Data, HasLen, 1)
	item, ok := x.Data[0].(SupervisorItem)
	c.Assert(ok, Equals, true)
	c.Assert(item.Name, Equals, "dummy")
	c.Assert(item.Pid, Equals, "31956")
}

func (s *SuperMockSuite) TestSupervisorInitError(c *C) {
	s.NewPlugin(ids.PluginID{"supervisor", "init_error"}, &SupervisorInitError{}, c)
	x := s.agent.GetData(c)
	c.Assert(x.NotApplicable, Equals, true)
}

func (s *SuperMockSuite) NewPlugin(
	id ids.PluginID, supervisor Supervisor, c *C,
) *SupervisorPlugin {
	v := NewSupervisorPlugin(id, s.agent)
	plugin, ok := v.(*SupervisorPlugin)
	c.Assert(ok, Equals, true)
	plugin.frequency = 1 * time.Millisecond
	plugin.supervisor = supervisor
	go plugin.Run()
	return plugin
}

type MockSupervisor struct {
}

func (self *MockSupervisor) Initialize() error {
	return nil
}

func (self *MockSupervisor) Processes() ([]SupervisorProcess, error) {
	proc := SupervisorProcess{
		Name:  "dummy",
		Pid:   31956,
		State: 20,
	}
	return []SupervisorProcess{proc}, nil
}

type SupervisorInitError struct {
}

func (self *SupervisorInitError) Initialize() error {
	return fmt.Errorf("Bummer!")
}

func (self *SupervisorInitError) Processes() ([]SupervisorProcess, error) {
	return nil, fmt.Errorf("Bummer!")
}

type SupervisorNoProcs struct {
}

func (self *SupervisorNoProcs) Initialize() error {
	return nil
}

func (self *SupervisorNoProcs) Processes() ([]SupervisorProcess, error) {
	return nil, fmt.Errorf("Bummer!")
}
