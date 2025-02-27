// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux
// +build linux

package linux

import (
	"io/ioutil"
	"os"
	"os/exec"
	"testing"
	"time"

	testHelper "github.com/newrelic/infrastructure-agent/internal/plugins/testing"

	"github.com/kolo/xmlrpc"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	log "github.com/sirupsen/logrus"

	"github.com/newrelic/infrastructure-agent/pkg/config"
	. "gopkg.in/check.v1"
)

const (
	SUPERVISOR_CONF      = "/tmp/supervisor_dummy.conf"
	SUPERVISOR_CONF_DATA = `
[supervisord]
logfile = /tmp/supervisor_dummy.log
pidfile = /tmp/supervisor_dummy.pid
minfds = 1024
identifier = supervisor_dummy

[supervisorctl]
serverurl = http://localhost:9123

[rpcinterface:supervisor]
supervisor.rpcinterface_factory = supervisor.rpcinterface:make_main_rpcinterface

[program:dummy]
command=sh /tmp/supervisor_dummy.sh

[unix_http_server]
file=/tmp/supervisor.sock.test   ; (the path to the socket file)
chmod=0777                       ; sockef file mode (default 0700)
`

	SUPERVISOR_DUMMY      = "/tmp/supervisor_dummy.sh"
	SUPERVISOR_DUMMY_DATA = `
sleep 10
`
)

// Register test suite
func TestSupervisor(t *testing.T) {
	TestingT(t)
}

type SupervisorSuite struct {
	client            *xmlrpc.Client
	agent             *testHelper.MockAgent
	config            *config.Config
	supervisorCommand *exec.Cmd
}

var _ = Suite(&SupervisorSuite{})

func (s *SupervisorSuite) SetUpSuite(c *C) {
	err := ioutil.WriteFile(
		SUPERVISOR_CONF,
		[]byte(SUPERVISOR_CONF_DATA),
		0666,
	)
	c.Assert(err, IsNil)
	err = ioutil.WriteFile(
		SUPERVISOR_DUMMY,
		[]byte(SUPERVISOR_DUMMY_DATA),
		0755,
	)
	c.Assert(err, IsNil)
	s.supervisorCommand = exec.Command("supervisord", "-n", "-c", SUPERVISOR_CONF)
	err = s.supervisorCommand.Start()
	if err != nil {
		c.Skip("No supervisor")
	}
	time.Sleep(2 * time.Second)
	client, err := xmlrpc.NewClient("unix:///RPC2", nil)
	c.Assert(err, IsNil)
	s.client = client

	s.agent = testHelper.NewMockAgent()
	v := NewSupervisorPlugin(ids.PluginID{"supervisor", "supervisor"}, s.agent)
	plugin, ok := v.(*SupervisorPlugin)
	c.Assert(ok, Equals, true)
	if !plugin.CanRun() {
		c.Skip("No supervisor")
	}
}

func (s *SupervisorSuite) TearDownSuite(c *C) {
	os.Remove(SUPERVISOR_CONF)
	os.Remove(SUPERVISOR_DUMMY)
	// kill supervisord

	if s.supervisorCommand.Process != nil {
		c.Assert(s.supervisorCommand.Process.Kill(), IsNil)
		_, err := s.supervisorCommand.Process.Wait()
		c.Assert(err, IsNil)
	}
}

func (s *SupervisorSuite) SetUpTest(c *C) {
	s.agent = testHelper.NewMockAgent()
}

func (s *SupervisorSuite) TestProcs(c *C) {
	client, err := NewSupervisorClient("unix", "/tmp/supervisor.sock.test")
	c.Assert(err, IsNil)
	procs, err := client.Processes()
	log.WithError(err).Error("can't get processes")
	c.Assert(err, IsNil)
	c.Assert(procs, NotNil)
	c.Assert(procs, HasLen, 1)
	c.Assert(procs[0].Name, Equals, "dummy")
}
