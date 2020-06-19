// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux
// +build !386

package helpers

import (
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	. "gopkg.in/check.v1"
)

type ServiceHelpersSuite struct{}

var _ = Suite(&ServiceHelpersSuite{})

func (s *ServiceHelpersSuite) TestGetStatusData(c *C) {
	statusBytes, err := ioutil.ReadFile(HostProc(strconv.Itoa(os.Getpid()), "status"))
	c.Assert(err, IsNil)

	data := getStatusData(statusBytes, []string{"Uid", "Gid", "PPid"})
	log.Info(data)
	c.Assert(data, HasLen, 3)
	c.Assert(data["Uid"], Not(IsNil))
	c.Assert(data["Gid"], Not(IsNil))
	c.Assert(data["PPid"], Not(IsNil))
}

func (self *ServiceHelpersSuite) TestGetPidUptime(c *C) {
	cmd := exec.Command("/bin/bash", "-c", `"sleep 15"`)
	c.Assert(cmd.Start(), IsNil)
	pid := cmd.Process.Pid

	lastUptime := time.Duration(0)
	for i := 0; i < 3; i++ {
		time.Sleep(1 * time.Second)
		startTime, err := getPidStartTime(pid)
		c.Assert(err, IsNil)
		uptime := time.Now().Sub(startTime)
		c.Assert(lastUptime == 0 || uptime-lastUptime >= 1*time.Second, Equals, true)
		lastUptime = uptime
	}

}

func (self *ServiceHelpersSuite) TestGetPidListenersTCP(c *C) {
	if os.Getuid() != 0 {
		c.Skip("Test can only be run as root")
	}
	cmd := exec.Command("/bin/nc", "-l", "-p", "12345")
	c.Assert(cmd.Start(), IsNil)
	defer func() {
		c.Assert(cmd.Process.Kill(), IsNil)
		err := cmd.Wait()
		c.Assert(err == nil || strings.Contains(err.Error(), "kill"), Equals, true)
	}()

	pid := cmd.Process.Pid
	time.Sleep(1 * time.Second)

	listeners, err := getPidListeners(pid)
	c.Assert(err, IsNil)
	c.Assert(listeners, Equals, `["TCP:0.0.0.0:12345"]`)
}

func (self *ServiceHelpersSuite) TestGetPidListenersUDP(c *C) {
	if os.Getuid() != 0 {
		c.Skip("Test can only be run as root")
	}
	cmd := exec.Command("/bin/nc", "-u", "-l", "-p", "12345")
	c.Assert(cmd.Start(), IsNil)
	defer func() {
		err := cmd.Process.Kill()
		if !(err == nil) {
			log.WithError(err).Error("error killing UDP nc listener")
		}
		err = cmd.Wait()
		if !(err == nil || strings.Contains(err.Error(), "kill")) {
			log.WithError(err).Error("error waiting on UDP nc listener")
		}
	}()

	pid := cmd.Process.Pid
	time.Sleep(1 * time.Second)

	listeners, err := getPidListeners(pid)
	c.Assert(err, IsNil)
	c.Assert(listeners, Equals, `["UDP:0.0.0.0:12345"]`)

	// verify cache is working
	listeners, err = getPidListenersWithCache(pid)
	c.Assert(err, IsNil)
	beforeCount := _pidListenersCounter
	listeners, err = getPidListenersWithCache(pid)
	c.Assert(listeners, Equals, `["UDP:0.0.0.0:12345"]`)
	c.Assert(_pidListenersCounter, Equals, beforeCount)
}
