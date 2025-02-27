// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux
// +build linux

package linux

import (
	"bufio"
	"bytes"
	"testing"

	. "gopkg.in/check.v1"
)

// Register test suite
func TestSystemmd(t *testing.T) {
	TestingT(t)
}

type SystemdSuite struct{}

var _ = Suite(&SystemdSuite{})

func (self *SystemdSuite) TestSystemctlParse(c *C) {
	data := `
auditd.service                                                                            loaded active   running Security Auditing Service
avahi-daemon.service                                                                      loaded active   running Avahi mDNS/DNS-SD Stack
brandbot.service                                                                          loaded inactive dead    Flexible Branding Service
cpupower.service                                                                          loaded inactive dead    Configure CPU power related settings
display-manager.service                                                                   not-found inactive dead    display-manager.service
exim.service                                                                              not-found inactive dead    exim.service
kdump.service                                                                             loaded failed   failed  Crash recovery kernel arming
crond.service                                                                             loaded failed   failed  Command Scheduler
kmod-static-nodes.service                                                                 loaded active   exited  Create list of required static device nodes for the current kernel
lvm2-monitor.service                                                                      loaded active   exited  Monitoring of LVM2 mirrors, snapshots etc. using dmeventd or progress polling
`

	scan := bufio.NewScanner(bytes.NewBuffer([]byte(data)))
	var findings []string
	for scan.Scan() {
		name, loaded, status := parseSystemctlOutput(scan.Text())
		if name == "" {
			c.Assert(status, Equals, "")
			c.Assert(loaded, Equals, "")
		} else if loaded == "active" && status == "running" {
			findings = append(findings, name)
		}
	}
	c.Assert(len(findings), Equals, 2)
	c.Assert(findings[0], Equals, "auditd")
	c.Assert(findings[1], Equals, "avahi-daemon")
}

func (self *SystemdSuite) TestGetPidFromName(c *C) {
	c.Assert(getPidFromName("MainPID=8575"), Equals, "8575")
	c.Assert(getPidFromName("MainPID=0"), Equals, "unknown")
	c.Assert(getPidFromName("MainPID="), Equals, "unknown")
	c.Assert(getPidFromName(" "), Equals, "unknown")
	c.Assert(getPidFromName(""), Equals, "unknown")
}
