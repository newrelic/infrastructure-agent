// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"io/ioutil"

	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	. "gopkg.in/check.v1"
)

type NewRelicInfraSuite struct{}

var _ = Suite(&NewRelicInfraSuite{})

func (s *NewRelicInfraSuite) TestLogRedirection(c *C) {
	logFile, err := ioutil.TempFile("", "newLogs.txt")
	c.Assert(err, IsNil)
	logText := "example logs here"
	_, _ = logFile.WriteString(logText)
	cfg := &config.Config{
		LogFile: logFile.Name(),
	}
	c.Assert(configureLogRedirection(cfg, &log.MemLogger{}), Equals, true)
	dat, err := ioutil.ReadFile(logFile.Name())
	c.Assert(err, IsNil)
	c.Assert(string(dat), Equals, logText)
}
