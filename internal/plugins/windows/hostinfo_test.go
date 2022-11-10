// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build windows
// +build windows

package windows

import (
	"github.com/newrelic/infrastructure-agent/internal/plugins/common"
	"testing"
	"time"

	testing2 "github.com/newrelic/infrastructure-agent/internal/plugins/testing"

	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"

	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/shirou/gopsutil/v3/host"
	. "gopkg.in/check.v1"
)

type HostinfoSuite struct {
	agent *testing2.MockAgent
}

var _ = Suite(&HostinfoSuite{})

func Test(t *testing.T) { TestingT(t) }

func (s *HostinfoSuite) SetUpSuite(c *C) {
}

func (s *HostinfoSuite) TearDownSuite(c *C) {
}

func (s *HostinfoSuite) SetUpTest(c *C) {
	s.agent = testing2.NewMockAgent()
}

func (s *HostinfoSuite) NewPlugin(id ids.PluginID, c *C) *HostinfoPlugin {
	cloudDetector := cloud.NewDetector(true, 0, 0, 0, false)
	v := NewHostinfoPlugin(id, s.agent,
		common.NewHostInfoCommon("testing", true, cloudDetector))
	plugin, ok := v.(*HostinfoPlugin)
	c.Assert(ok, Equals, true)
	go plugin.Run()
	return plugin
}

func (s *HostinfoSuite) TestDataGathering(c *C) {
	now := time.Now().Unix()

	plugin := s.NewPlugin(ids.PluginID{"win", "test1"}, c)
	c.Assert(plugin, NotNil)
	info := &host.InfoStat{
		OS:              "windows",
		Platform:        "platform",
		PlatformFamily:  "family",
		PlatformVersion: "version",
		BootTime:        uint64(now),
	}
	data := plugin.gatherHostinfo(plugin.Context, info)

	c.Assert(data, NotNil)
}

func (s *HostinfoSuite) TestCPUAndRamInfo(c *C) {
	cpuInfo := getCpuInfo()
	c.Assert(cpuInfo, NotNil)

	ram := getRam()
	c.Assert(ram, NotNil)
}
