// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux || darwin
// +build linux darwin

package linux

import (
	"bufio"
	"bytes"
	"testing"

	. "gopkg.in/check.v1"
)

// Register test suite.
func TestDaemontools(t *testing.T) {
	t.Parallel()
}

// boilerplate

type DaemontoolsSuite struct{}

var _ = Suite(&DaemontoolsSuite{})

func (ds *DaemontoolsSuite) TestSvstatParse(c *C) {
	table := []struct {
		output string
		up     bool
		pid    int64
	}{
		{"pewp: up (pid 7101) 1 seconds", true, 7101},
		{"pewp: down 7 seconds, normally up", false, 0},
	}

	for _, row := range table {
		up, pid, err := parseSvstatOutput(row.output)
		c.Assert(err, IsNil)
		c.Check(up, Equals, row.up)
		c.Check(pid, Equals, row.pid)
	}
}

func (ds *DaemontoolsSuite) TestPsLineParse(c *C) {
	data := `
 1399 vim state_manager.go +924
 2136 sshd: vagrant [priv]
 2151 sshd: vagrant@pts/0 
 2152 -bash
 2213 tmux
 2214 -bash
 2490 tmux attach
 2596 -bash
 2753 -bash
 3042 vim plugins/daemontools.go
 5350 [kworker/0:2]
 5545 sudo su
 5546 su
 5547 bash
 7101 /bin/bash ./run
 7445 [kworker/0:0]
 9127 [kworker/0:3]
10206 [kworker/0:1]
10550 sleep 5
10551 ps -e -o pid,args
11473 -bash
12514 gocode -s -sock unix -addr localhost:37373
19797 -bash
20930 /bin/sh /usr/bin/svscanboot
20932 svscan /etc/service
20933 readproctitle service errors: ...E?WHEE?WHEE?WHEE?WHEE?WHEE?WHEE?WHEE?WHEE?WHEE?WHEE?WHEE?WHEE?WHEE?WHEE?WHEE?WHEE?WHEE?WHEE?WHEE?WHEE?WHEE?WHEE?WHEE?WHEE?WHEE?WHEE?WHEE?WHEE?WHEE?WHEE?WHEE?WHEE?WH
20979 vim partition_util_test.go
 1258 supervise pewp
29250 -bash
32195 -bash
`

	scan := bufio.NewScanner(bytes.NewBuffer([]byte(data)))
	var findings []int64
	for scan.Scan() {
		pid, _, err := parseSuperviseProcessListing(scan.Text())
		c.Assert(err, IsNil)
		if pid != 0 {
			findings = append(findings, pid)
		}
	}
	c.Assert(len(findings), Equals, 1)
	c.Assert(findings[0], Equals, int64(1258))
}
