// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux || darwin
// +build linux darwin

package linux

import (
	"github.com/newrelic/infrastructure-agent/internal/plugins/common"
	"io/ioutil"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/os/distro"
	"github.com/newrelic/infrastructure-agent/internal/os/fs"
	testing2 "github.com/newrelic/infrastructure-agent/internal/plugins/testing"

	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"
	"github.com/stretchr/testify/assert"

	. "github.com/newrelic/infrastructure-agent/pkg/go-better-check"
	. "gopkg.in/check.v1"
)

type HostinfoSuite struct {
	agent *testing2.MockAgent
}

var _ = Suite(&HostinfoSuite{})

var osRelease string

func (s *HostinfoSuite) SetUpSuite(c *C) {
}

func (s *HostinfoSuite) TearDownSuite(c *C) {
}

func (s *HostinfoSuite) SetUpTest(c *C) {
	s.agent = testing2.NewMockAgent()
}

func (s *HostinfoSuite) NewPlugin(c *C) *HostinfoPlugin {
	cloudDetector := cloud.NewDetector(true, 0, 0, 0, false)
	v := NewHostinfoPlugin(s.agent, common.NewHostInfoCommon("test", true, cloudDetector))

	plugin, ok := v.(*HostinfoPlugin)
	c.Assert(ok, Equals, true)
	go plugin.Run()
	return plugin
}

func (s *HostinfoSuite) TestReadProcFile(c *C) {
	err := ioutil.WriteFile("/tmp/cpuinfo", []byte(cpuinfo), 0644)
	c.Assert(err, IsNil)

	cpuName := readProcFile("/tmp/cpuinfo", regexp.MustCompile(`model\sname\s*:\s`))
	cpuNum := readProcFile("/tmp/cpuinfo", regexp.MustCompile(`cpu\scores\s*:\s`))

	c.Assert(cpuName, Equals, "Intel(R) Core(TM) i7-4790K CPU @ 4.00GHz")
	c.Assert(cpuNum, Equals, "2")
}

func (s *HostinfoSuite) TestRunCmd(c *C) {
	output := runCmd("echo", "test")

	c.Assert(output, Equals, "test\n")
}

func (s *HostinfoSuite) TestGetDistro(c *C) {
	name := distro.GetDistro()
	c.Check(name, Not(Equals), "")

	cloudDetector := cloud.NewDetector(true, 0, 0, 0, false)

	v := NewHostinfoPlugin(s.agent, common.NewHostInfoCommon("test", true, cloudDetector))
	plugin, ok := v.(*HostinfoPlugin)
	c.Assert(ok, Equals, true)
	data := plugin.Data()
	c.Assert(data, HasLen, 1)
	hostInfo, ok := data[0].(*HostInfoLinux)
	c.Assert(ok, Equals, true)
	c.Assert(hostInfo.Distro, HasPrefix, name)
}

func (s *HostinfoSuite) TestGetTotalCpu(c *C) {
	err := ioutil.WriteFile("/tmp/cpuinfo", []byte(cpuinfo), 0644)
	c.Assert(err, IsNil)
	c.Assert(getTotalCpu("/tmp/cpuinfo"), Equals, "2")
}

func (s *HostinfoSuite) TestGetCpuNum(c *C) {
	err := ioutil.WriteFile("/tmp/cpuinfo", []byte(cpuinfo), 0644)
	c.Assert(err, IsNil)
	c.Assert(getCpuNum("/tmp/cpuinfo", "1"), Equals, "2")
}

func (s *HostinfoSuite) TestGetCpuNumFallback(c *C) {
	err := os.Remove("/tmp/cpuinfo")
	c.Assert(err, IsNil)
	c.Assert(getCpuNum("/tmp/cpuinfo", "1"), Equals, "1")
}

func (s *HostinfoSuite) TestLsbRelease(c *C) {
	err := ioutil.WriteFile("/tmp/lsb_release", []byte(lsbRelease), 0644)
	c.Assert(err, IsNil)
	release, err := fs.ReadFileFieldMatching("/tmp/lsb_release", regexp.MustCompile(`DISTRIB_DESCRIPTION="(.*?)"`))
	c.Assert(err, IsNil)
	c.Assert(release, Equals, "Ubuntu 14.04.5 LTS")
}

var cpuinfo = `
processor	: 0
vendor_id	: GenuineIntel
cpu family	: 6
model		: 60
model name	: Intel(R) Core(TM) i7-4790K CPU @ 4.00GHz
stepping	: 3
microcode	: 0x19
cpu MHz		: 4019.576
cache size	: 6144 KB
physical id	: 0
siblings	: 1
core id		: 0
cpu cores	: 2
apicid		: 0
initial apicid	: 0
fpu		: yes
fpu_exception	: yes
cpuid level	: 5
wp		: yes
flags		: fpu vme de pse tsc msr pae mce cx8 apic sep mtrr pge mca cmov pat pse36 clflush mmx fxsr sse sse2 syscall nx rdtscp lm constant_tsc rep_good nopl pni monitor ssse3 lahf_lm
bogomips	: 8039.15
clflush size	: 64
cache_alignment	: 64
address sizes	: 39 bits physical, 48 bits virtual
power management:

processor       : 1
vendor_id       : GenuineIntel
cpu family      : 6
model           : 70
model name      : Intel(R) Core(TM) i7-4870HQ CPU @ 2.50GHz
stepping        : 1
microcode       : 0x13
cpu MHz         : 2494.640
cache size      : 6144 KB
physical id     : 2
siblings        : 1
core id         : 0
cpu cores       : 2
apicid          : 2
initial apicid  : 2
fpu             : yes
fpu_exception   : yes
cpuid level     : 13
wp              : yes
flags           : fpu vme de pse tsc msr pae mce cx8 apic sep mtrr pge mca cmov pat pse36 clflush dts mmx fxsr sse sse2 ss syscall nx pdpe1gb rdtscp lm constant_tsc arch_perfmon pebs bts nopl xtopology tsc_reliable nonstop_tsc aperfmperf pni pclmulqdq ssse3 fma cx16 pcid sse4_1 sse4_2 x2apic movbe popcnt aes xsave avx f16c rdrand hypervisor lahf_lm ida arat epb pln pts dtherm fsgsbase smep
bogomips        : 4989.28
clflush size    : 64
cache_alignment : 64
address sizes   : 40 bits physical, 48 bits virtual
power management:
`

var lsbRelease = `
DISTRIB_ID=Ubuntu
DISTRIB_RELEASE=14.04
DISTRIB_CODENAME=trusty
DISTRIB_DESCRIPTION="Ubuntu 14.04.5 LTS"
`

func Test_getUpSince(t *testing.T) {
	_, err := time.Parse("2006-01-02 15:04:05", getUpSince())
	assert.NoError(t, err)
}
