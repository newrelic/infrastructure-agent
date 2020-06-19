// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build windows

package windows

import (
	testing2 "github.com/newrelic/infrastructure-agent/internal/plugins/testing"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"

	"github.com/shirou/gopsutil/host"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"

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
	v := NewHostinfoPlugin(id, s.agent, cloudDetector)
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

type fakeHarvester struct {
	mock.Mock
}

// GetInstanceID will return the id of the cloud instance.
func (f *fakeHarvester) GetInstanceID() (string, error) {
	args := f.Called()
	return args.String(0), args.Error(1)
}

// GetHostType will return the cloud instance type.
func (f *fakeHarvester) GetHostType() (string, error) {
	args := f.Called()
	return args.String(0), args.Error(1)
}

// GetCloudType will return the cloud type on which the instance is running.
func (f *fakeHarvester) GetCloudType() cloud.Type {
	args := f.Called()
	return args.Get(0).(cloud.Type)
}

// Returns a string key which will be used as a HostSource (see host_aliases plugin).
func (f *fakeHarvester) GetCloudSource() string {
	args := f.Called()
	return args.String(0)
}

// GetRegion returns the cloud region
func (f *fakeHarvester) GetRegion() (string, error) {
	args := f.Called()
	return args.String(0), args.Error(1)
}

// GetHarvester returns instance of the Harvester detected (or instance of themselves)
func (f *fakeHarvester) GetHarvester() (cloud.Harvester, error) {
	return f, nil
}

func TestHostinfoPluginSetCloudRegion(t *testing.T) {
	testCases := []struct {
		name       string
		assertions func(*HostinfoData)
		setMock    func(*fakeHarvester)
	}{
		{
			name: "no cloud",
			assertions: func(d *HostinfoData) {
				assert.Equal(t, "", d.RegionAWS)
				assert.Equal(t, "", d.RegionAzure)
				assert.Equal(t, "", d.RegionGCP)
				assert.Equal(t, "", d.RegionAlibaba)
			},
			setMock: func(h *fakeHarvester) {
				h.On("GetCloudType").Return(cloud.TypeNoCloud)
			},
		},
		{
			name: "cloud aws",
			assertions: func(d *HostinfoData) {
				assert.Equal(t, "us-east-1", d.RegionAWS)
				assert.Equal(t, "", d.RegionAzure)
				assert.Equal(t, "", d.RegionGCP)
				assert.Equal(t, "", d.RegionAlibaba)
			},
			setMock: func(h *fakeHarvester) {
				h.On("GetCloudType").Return(cloud.TypeAWS)
				h.On("GetRegion").Return("us-east-1", nil)
			},
		},
		{
			name: "cloud azure",
			assertions: func(d *HostinfoData) {
				assert.Equal(t, "", d.RegionAWS)
				assert.Equal(t, "us-east-1", d.RegionAzure)
				assert.Equal(t, "", d.RegionGCP)
				assert.Equal(t, "", d.RegionAlibaba)
			},
			setMock: func(h *fakeHarvester) {
				h.On("GetCloudType").Return(cloud.TypeAzure)
				h.On("GetRegion").Return("us-east-1", nil)
			},
		},
		{
			name: "cloud gcp",
			assertions: func(d *HostinfoData) {
				assert.Equal(t, "", d.RegionAWS)
				assert.Equal(t, "", d.RegionAzure)
				assert.Equal(t, "us-east-1", d.RegionGCP)
				assert.Equal(t, "", d.RegionAlibaba)
			},
			setMock: func(h *fakeHarvester) {
				h.On("GetCloudType").Return(cloud.TypeGCP)
				h.On("GetRegion").Return("us-east-1", nil)
			},
		}, {
			name: "cloud alibaba",
			assertions: func(d *HostinfoData) {
				assert.Equal(t, "", d.RegionAWS)
				assert.Equal(t, "", d.RegionAzure)
				assert.Equal(t, "", d.RegionGCP)
				assert.Equal(t, "ap-southeast-2", d.RegionAlibaba)
			},
			setMock: func(h *fakeHarvester) {
				h.On("GetCloudType").Return(cloud.TypeAlibaba)
				h.On("GetRegion").Return("ap-southeast-2", nil)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			h := new(fakeHarvester)
			testCase.setMock(h)
			data := &HostinfoData{}
			p := &HostinfoPlugin{
				PluginCommon: agent.PluginCommon{
					ID:      ids.HostInfo,
					Context: testing2.NewMockAgent(),
				},
				cloudHarvester: h,
			}
			p.setCloudRegion(data)
			testCase.assertions(data)
			h.AssertExpectations(t)
		})
	}
}
